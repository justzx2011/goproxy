package tunnel

import (
	"errors"
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

func (t *Tunnel) main () {
	var err error
	var buf []byte
	var ev uint8
	var loop bool = true

QUIT:
	for loop {
		select {
		case ev = <- t.c_evin:
			if ev == END { break QUIT }
			err = t.on_event(ev)
		case <- t.connest: err = t.Close()
		case <- t.retrans: err = t.on_retrans()
		case <- t.delayack:
			err = t.send(ACK, []byte{})
			t.delayack = nil
		case <- t.keepalive: err = t.Close()
		case <- t.finwait: err = t.Close()
		case <- t.timewait: err = t.Close()
		case buf = <- t.c_recv: err = t.on_data(buf)
		case buf = <- t.c_write: err = t.send(0, buf)
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
		t.connest = time.After(time.Duration(TM_CONNEST) * time.Second)
		t.status = SYNSENT
		return t.send(SYN, []byte{})
	case FIN:
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

	if DEBUG { log.Println("recv in", t.Dump()) }
	pkt, err = Unpack(buf)
	if err != nil { return }

	if DEBUG { log.Println("recv packet", pkt.Dump()) }
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

	if DEBUG { log.Println("recv out", t.Dump()) }
	return
}

func (t *Tunnel) ack_recv(pkt *Packet) (err error) {
	var ti time.Time = time.Now()
	var p *Packet

	for len(t.sendbuf) != 0 && t.sendbuf[0].seq >= pkt.ack {
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
