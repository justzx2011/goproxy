package tunnel

import (
	"bytes"
	"container/heap"
	"errors"
	"fmt"
	"log"
	"net"
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
	sendbuf PacketHeap
	recvbuf PacketHeap

	rtt uint32
	rttvar uint32
	onclose func ()

	connest <-chan time.Time
	resend <-chan time.Time
	delayack <-chan time.Time
	keepalive <-chan time.Time
	finwait <-chan time.Time
	timewait <-chan time.Time

	c_read chan []byte
	c_write chan []byte
	c_evin chan uint8
	c_evout chan uint8
}

func NewTunnel(remote *net.UDPAddr) (t *Tunnel, err error) {
	t = new(Tunnel)
	t.remote = remote
	t.status = CLOSED

	t.c_recv = make(chan []byte, 10)

	t.sendseq = 0
	t.recvseq = 0
	t.recvack = 0
	t.sendbuf = make(PacketHeap, 0)
	t.recvbuf = make(PacketHeap, 0)

	t.rtt = 200000
	t.rttvar = 200000
	t.keepalive = time.After(time.Duration(KEEPALIVE) * time.Second)

	t.c_read = make(chan []byte, 10)
	t.c_write = make(chan []byte, 10)
	t.c_evin = make(chan uint8)
	t.c_evout = make(chan uint8)

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
	var loop bool = true

	for loop {
		select {
		case buf = <- t.c_recv: err = t.on_data(buf)
		case buf = <- t.c_write: err = t.send(0, buf)
		case ev = <- t.c_evin:
			if ev == END {
				loop = false
			}else{ err = t.on_event(ev) }
		case <- t.connest: err = t.Close()
		case <- t.resend:
		case <- t.delayack: err = t.send(ACK, []byte{})
		case <- t.keepalive: err = t.Close()
		case <- t.finwait: err = t.Close()
		case <- t.timewait: err = t.Close()
		}
		if err != nil { log.Println(err.Error()) }
	}
}

func (t *Tunnel) on_event (ev uint8) (err error) {
	switch ev {
	case SYN:
		if t.status != CLOSED {
			return errors.New("somebody try to connect, " + t.Dump())
		}
		t.connest = time.After(time.Duration(CONNEST) * time.Second)
		t.status = SYNSENT
		return t.send(SYN, []byte{})
	case FIN:
		if t.status != EST { return }
		t.finwait = time.After(time.Duration(FINWAIT_2) * time.Millisecond)
		t.status = FINWAIT
		return t.send(FIN, []byte{})
	}
	return errors.New("unknown event")
}

func (t *Tunnel) on_data(buf []byte) (err error) {
	var pkt *Packet
	var p *Packet

	if DEBUG { log.Println("recv in", t.Dump()) }
	pkt, err = Unpack(buf)
	if err != nil { return }

	if DEBUG { log.Println("recv packet", pkt.Dump()) }
	t.keepalive = time.After(time.Duration(KEEPALIVE) * time.Second)

	if (pkt.flag & ACK) != 0 {
		err = t.ack_recv(pkt)
		if err != nil { return }
	}

	switch{
	case (pkt.seq - t.recvseq) < 0: return 
	case (pkt.seq - t.recvseq) == 0:
		for p = pkt; ; {
			err = t.proc_packet(p)
			if err != nil { return }

			if len(t.recvbuf) == 0 { break }
			if t.recvbuf[0].seq != t.recvseq { break }
			p = heap.Pop(&t.recvbuf).(*Packet)
		}
	case (pkt.seq - t.recvseq) > 0: heap.Push(&t.recvbuf, pkt)
	}

	if t.recvseq != t.recvack {
		t.delayack = time.After(time.Duration(DELAYACK) * time.Millisecond)
	}

	if DEBUG { log.Println("recv out", t.Dump()) }
	return
}

func (t *Tunnel) proc_packet(pkt *Packet) (err error) {
	if (pkt.flag & ACK) != 0 {
		switch t.status {
		case SYNRCVD:
			t.status = EST
		case LASTACK:
			t.status = CLOSED
			t.Close()
		}
	}

	if (pkt.flag & SYN) != 0 { return t.proc_syn(pkt) }
	if (pkt.flag & FIN) != 0 { return t.proc_fin(pkt) }

	if len(pkt.content) > 0 {
		t.recvseq += int32(len(pkt.content))
		t.c_read <- pkt.content
	}else if (pkt.flag & ^ACK) != 0 {
		t.recvseq += 1
	}

	return
}

func (t *Tunnel) proc_syn (pkt *Packet) (err error) {
	t.recvseq += 1
	if (pkt.flag & ACK) != 0 {
		if t.status != SYNSENT {
			return errors.New("status wrong, SYN ACK, " + t.Dump())
		}
		t.connest = nil
		t.status = EST
		err = t.send(ACK, []byte{})
		if err != nil { return }
		t.c_evout <- SYN
	}else{
		if t.status != CLOSED {
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
			return errors.New("status wrong, FIN ACK, " + t.Dump())
		}else{ t.finwait = nil }
		t.status = TIMEWAIT
		err = t.send(ACK, []byte{})
		if err != nil { return }

		t.timewait = time.After(time.Duration(2 * MSL) * time.Millisecond)
		t.c_evout <- FIN
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
			t.timewait = time.After(time.Duration(2 * MSL) * time.Millisecond)
		default:
			return errors.New("status wrong, FIN, " + t.Dump())
		}
	}
	return
}

func (t *Tunnel) ack_recv(pkt *Packet) (err error) {
	var ti time.Time = time.Now()
	var p *Packet

	for len(t.sendbuf) != 0 && t.sendbuf[0].seq >= pkt.ack {
		p = heap.Pop(&t.sendbuf).(*Packet)

		delta := int32(ti.Sub(p.t).Nanoseconds() / 1000) - int32(t.rtt)
		t.rtt = uint32(int32(t.rtt) + delta >> 3)
		if delta < 0 { delta = -delta }
		t.rttvar = uint32(int32(t.rttvar) + (delta - int32(t.rttvar)) >> 2)
	}
	
	// for _, p := range t.sendbuf {
	// 	if p.seq >= pkt.ack {
	// 		sendbuf = append(sendbuf, p)
	// 	}else{
	// 		delta := int32(ti.Sub(p.t).Nanoseconds() / 1000) - int32(t.rtt)
	// 		t.rtt = uint32(int32(t.rtt) + delta >> 3)
	// 		if delta < 0 { delta = -delta }
	// 		t.rttvar = uint32(int32(t.rttvar) + (delta - int32(t.rttvar)) >> 2)
	// 		p.timeout.Stop()
	// 	}
	// }
	// t.sendbuf = sendbuf

	if t.resend != nil {
		// clean up resend chan
	}
	return
}

func (t *Tunnel) send(flag uint8, content []byte) (err error) {
	if t.recvack != t.recvseq { flag |= ACK }
	err = t.send_packet(NewPacket(t, flag, content))
	if err != nil { return }

	if len(content) > 0 {
		t.sendseq += int32(len(content))
	}else if (flag & ^ACK) != 0 {
		t.sendseq += 1
	}
	if DEBUG { log.Println("send out", t.Dump()) }

	t.recvack = t.recvseq
	if t.delayack != nil { t.delayack = nil }
	return
}

func (t *Tunnel) send_packet(pkt *Packet) (err error) {
	var buf []byte
	if DEBUG { log.Println("send in", pkt.Dump()) }

	buf, err = pkt.Pack()
	if err != nil { return }
	if DEBUG { log.Println("send", t.remote, buf) }

	t.c_send <- &DataBlock{t.remote, buf}
	if (pkt.flag & ^ACK) == 0 && len(pkt.content) == 0 { return }

	pkt.t = time.Now()
	heap.Push(&t.sendbuf, pkt)

	if t.resend == nil {
		d := time.Duration(t.rtt + t.rttvar >> 2) * time.Microsecond
		t.resend = time.After(time.Duration(d) * time.Second)
	}

	// pkt.timeout = time.AfterFunc(d, func () {
	// 	pkt.resend_count += 1
	// 	if pkt.resend_count > MAXRESEND {
	// 		log.Println("send packet more then maxresend times")
	// 		t.Close()
	// 	}else{ t.send_packet(pkt) }
	// })

	return
}

func (t *Tunnel) resend () (err error) {
	// FIXME: how to resend
	pkt.resend_count += 1
	if pkt.resend_count > MAXRESEND {
		log.Println("send packet more then maxresend times")
		t.Close()
	}else{ t.send_packet(pkt) }

	// todo: next resend
}

func (t *Tunnel) Close () (err error) {
	if t.onclose != nil { t.onclose() }
	t.c_evin <- END
	close(t.c_send)
	return
}