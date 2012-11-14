package tunnel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

type Tunnel struct {
	remote *net.UDPAddr
	status uint8

	c_recv chan []byte
	c_send chan *DataBlock

	sendseq int32
	recvseq int32
	recvack int32
	sendbuf PacketQueue
	recvbuf PacketQueue

	rtt uint32
	rttvar uint32
	sack_count uint
	retrans_count uint
	onclose func ()

	connest <-chan time.Time
	retrans <-chan time.Time
	delayack <-chan time.Time
	keepalive <-chan time.Time
	finwait <-chan time.Time
	timewait <-chan time.Time

	c_read chan []byte
	c_write chan []byte
	c_evin chan uint8
	c_evout chan uint8
}

func NewTunnel(remote *net.UDPAddr) (t *Tunnel) {
	t = new(Tunnel)
	t.remote = remote
	t.status = CLOSED

	t.c_recv = make(chan []byte, 1)

	t.sendseq = 0
	t.recvseq = 0
	t.recvack = 0
	t.sendbuf = make(PacketQueue, 0)
	t.recvbuf = make(PacketQueue, 0)

	t.rtt = 200000
	t.rttvar = 200000
	t.sack_count = 0
	t.retrans_count = 0
	t.keepalive = time.After(time.Duration(TM_KEEPALIVE) * time.Second)

	t.c_read = make(chan []byte, 1)
	t.c_write = make(chan []byte, 1)
	t.c_evin = make(chan uint8, 1)
	t.c_evout = make(chan uint8, 1)

	go t.main()
	return
}

func (t Tunnel) Dump() string {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprintf(buf,
		"status: %s, sendseq: %d, recvseq: %d, sendbuf: %d, recvbuf: %d, readbuf: %d, writebuf: %d",
		DumpStatus(t.status), t.sendseq, t.recvseq,
		len(t.sendbuf), len(t.recvbuf), len(t.c_read), len(t.c_write))
	return buf.String()
}

func (t *Tunnel) main () {
	var err error
	var buf []byte
	var ev uint8

	defer func () {
		logger.Debug("tunnel main quit")
		t.status = CLOSED
		for len(t.c_read) != 0 { <- t.c_read }
		close(t.c_read)
		if len(t.c_evout) == 0 { t.c_evout <- EV_CLOSED }
		if t.onclose != nil { t.onclose() }
	}()

QUIT:
	for {
		select {
		case ev = <- t.c_evin:
			if ev == EV_END { break QUIT }
			err = t.on_event(ev)
		case <- t.connest:
			logger.Debug("timer connest")
			t.c_evin <- EV_END
		case <- t.retrans:
			logger.Debug("timer retrans")
			err = t.on_retrans()
		case <- t.delayack:
			logger.Debug("timer delayack")
			err = t.send(ACK, []byte{})
		case <- t.keepalive:
			logger.Debug("timer keepalive")
			t.c_evin <- EV_END
		case <- t.finwait:
			logger.Debug("timer finwait")
			t.c_evin <- EV_END
		case <- t.timewait:
			logger.Debug("timer timewait")
			t.c_evin <- EV_END
		// case len(t.c_read) == 0 && buf = <- t.c_recv:
		case buf = <- t.c_recv: err = t.on_data(buf)
		// case len(t.c_send) == 0 && buf = <- t.c_write:
		case buf = <- t.c_write: err = t.send(0, buf)
		}
		if err != nil { logger.Err(err.Error()) }
		logger.Debug("loop end " + t.Dump())
	}
}

func (t *Tunnel) on_event (ev uint8) (err error) {
	if logger != nil { logger.Debug("on event " + strconv.Itoa(int(ev))) }
	switch ev {
	case EV_CONNECT:
		if t.status != CLOSED {
			return errors.New("somebody try to connect, " + t.Dump())
		}
		t.connest = time.After(time.Duration(TM_CONNEST) * time.Second)
		t.status = SYNSENT
		return t.send(SYN, []byte{})
	case EV_CLOSE:
		if t.status != EST { return }
		t.finwait = time.After(time.Duration(TM_FINWAIT) * time.Millisecond)
		t.status = FINWAIT
		return t.send(FIN, []byte{})
	}
	return errors.New("unknown event")
}

func (t *Tunnel) on_data(buf []byte) (err error) {
	var pkt *Packet
	var p *Packet

	pkt, err = Unpack(buf)
	if err != nil { return }

	if logger != nil { logger.Debug("recv packet " + pkt.Dump()) }
	t.keepalive = time.After(time.Duration(TM_KEEPALIVE) * time.Second)

	if (pkt.flag & ACK) != 0 {
		err = t.ack_recv(pkt)
		if err != nil { return }
	}
	if pkt.flag == ACK { t.sack_count = 0 }

	switch{
	case (pkt.seq - t.recvseq) < 0: return 
	case (pkt.seq - t.recvseq) == 0:
		for p = pkt; ; {
			err = t.proc_packet(p)
			if err != nil { return }

			if len(t.recvbuf) == 0 { break }
			if t.recvbuf[0].seq != t.recvseq { break }
			p = t.recvbuf.Pop()
		}
	case (pkt.seq - t.recvseq) > 0:
		t.recvbuf.Push(pkt)
		err = t.send_sack()
	}

	if t.recvseq != t.recvack && t.delayack == nil {
		t.delayack = time.After(time.Duration(TM_DELAYACK) * time.Millisecond)
	}

	return
}

func (t *Tunnel) proc_packet(pkt *Packet) (err error) {
	if t.status == SYNRCVD { t.status = EST }
	if (pkt.flag & ACK) != 0 {
		if t.status == LASTACK { t.c_evin <- EV_END }
	}

	if (pkt.flag & SYN) != 0 { return t.proc_syn(pkt) }
	if (pkt.flag & FIN) != 0 { return t.proc_fin(pkt) }
	if (pkt.flag & SACK) != 0 { return t.proc_sack(pkt) }

	if len(pkt.content) > 0 {
		t.recvseq += int32(len(pkt.content))
		t.c_read <- pkt.content
	}else if pkt.flag != ACK {
		t.recvseq += 1
	}

	return
}

func (t *Tunnel) proc_syn (pkt *Packet) (err error) {
	t.recvseq += 1
	if (pkt.flag & ACK) != 0 {
		if t.status != SYNSENT {
			t.c_evin <- EV_END
			return errors.New("status wrong, SYN ACK, " + t.Dump())
		}
		t.connest = nil
		t.status = EST
		err = t.send(ACK, []byte{})
		if err != nil { return }
		t.c_evout <- EV_CONNECTED
	}else{
		if t.status != CLOSED {
			t.c_evin <- EV_END
			return errors.New("status wrong, SYN, " + t.Dump())
		}
		t.status = SYNRCVD
		err = t.send(SYN | ACK, []byte{})
		if err != nil { return }
	}
	return
}

func (t *Tunnel) proc_fin (pkt *Packet) (err error) {
	t.recvseq += 1
	if (pkt.flag & ACK) != 0 {
		if t.status != FINWAIT {
			t.c_evin <- EV_END
			return errors.New("status wrong, FIN ACK, " + t.Dump())
		}else{ t.finwait = nil }
		t.status = TIMEWAIT
		err = t.send(ACK, []byte{})
		if err != nil { return }

		t.timewait = time.After(2 * time.Duration(TM_MSL) * time.Millisecond)
		t.c_evout <- EV_CLOSED
	}else{
		switch t.status {
		case EST:
			t.status = LASTACK
			err = t.send(FIN | ACK, []byte{})
			if err != nil { return }
		case FINWAIT:
			t.finwait = nil
			t.status = TIMEWAIT
			err = t.send(ACK, []byte{})
			if err != nil { return }
			
			// wait 2*MSL to run close
			t.timewait = time.After(2 * time.Duration(TM_MSL) * time.Millisecond)
		default:
			t.c_evin <- EV_END
			return errors.New("status wrong, FIN, " + t.Dump())
		}
	}
	return
}

func (t *Tunnel) proc_sack(pkt *Packet) (err error) {
	var id int32
	var sendbuf PacketQueue
	logger.Warning("proc sack")
	buf := bytes.NewBuffer(pkt.content)

	binary.Read(buf, binary.BigEndian, &id)
	for _, p := range t.sendbuf {
		if p.seq == id {
			err = binary.Read(buf, binary.BigEndian, &id)
			if err == io.EOF {
				err = nil
				break
			}
			if err != nil { return }
		}else{ sendbuf = append(sendbuf, p) }
	}
	t.sendbuf = sendbuf

	t.sack_count += 1
	if t.sack_count > RETRANS_SACKCOUNT {
		logger.Warning("sack resend")
		t.resend(id, true)
		t.sack_count = 0
	}

	return
}

func (t *Tunnel) ack_recv(pkt *Packet) (err error) {
	var ti time.Time = time.Now()
	var p *Packet

	for len(t.sendbuf) != 0 && t.sendbuf[0].seq < pkt.ack {
		p = t.sendbuf.Pop()

		delta := int32(ti.Sub(p.t).Nanoseconds() / 1000) - int32(t.rtt)
		t.rtt = uint32(int32(t.rtt) + delta >> 3)
		if delta < 0 { delta = -delta }
		t.rttvar = uint32(int32(t.rttvar) + (delta - int32(t.rttvar)) >> 2)
	}

	t.retrans_count = 0
	if t.retrans != nil {
		if len(t.sendbuf) == 0 {
			t.retrans = nil
		}else{
			d := time.Duration(t.rtt + t.rttvar << 2) 
			d -= ti.Sub(t.sendbuf[0].t)
			t.retrans = time.After(d * time.Microsecond)
		}
	}
	return
}

func (t *Tunnel) send_sack() (err error) {
	logger.Warning("send sack")
	buf := bytes.NewBuffer([]byte{})
	for i, p := range t.recvbuf {
		if i > 0x7f { break }
		binary.Write(buf, binary.BigEndian, p.seq)
	}
	return t.send(SACK, buf.Bytes())
}

func (t *Tunnel) send(flag uint8, content []byte) (err error) {
	if t.recvack != t.recvseq { flag |= ACK }
	err = t.send_packet(NewPacket(t, flag, content), true)
	if err != nil { return }

	switch {
	case (flag & SACK) != 0:
	case len(content) > 0: t.sendseq += int32(len(content))
	case flag != ACK: t.sendseq += 1
	}

	t.recvack = t.recvseq
	if t.delayack != nil { t.delayack = nil }

	if logger != nil { logger.Debug("send out " + t.Dump()) }
	return
}

func (t *Tunnel) send_packet(pkt *Packet, retrans bool) (err error) {
	var buf []byte
	if logger != nil { logger.Debug("send in " + pkt.Dump()) }

	buf, err = pkt.Pack()
	if err != nil { return }

	t.c_send <- &DataBlock{t.remote, buf}
	if (pkt.flag & SACK) != 0 { return }
	if pkt.flag == ACK && len(pkt.content) == 0 { return }
	if !retrans { return }

	pkt.t = time.Now()
	t.sendbuf.Push(pkt)

	if t.retrans == nil {
		// WARN: is this right?
		d := time.Duration(t.rtt + t.rttvar << 2)
		t.retrans = time.After(d * time.Microsecond)
	}
	return
}

func (t *Tunnel) resend (stopid int32, stop bool) (err error) {
	for _, p := range t.sendbuf {
		err = t.send_packet(p, false)
		if err != nil { return }
		if stop && (p.seq - stopid) >= 0 { return }
	}
	return
}

func (t *Tunnel) on_retrans () (err error) {
	t.retrans_count += 1
	if t.retrans_count > MAXRESEND {
		logger.Err("send packet more then maxretrans times")
		t.c_evin <- EV_END
		return
	}

	err = t.resend(0, false)
	if err != nil { return }

	d := (t.rtt + t.rttvar << 2) * (1 << t.retrans_count)
	t.retrans = time.After(time.Duration(d) * time.Microsecond)
	return
}
