package tunnel

import (
	"bytes"
	"container/heap"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"time"
)

const MSL = 1000

// timeout when closing

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

	D uint32
	rtt uint32
	onclose func ()

	c_read chan []byte
	c_write chan []byte
	c_closing chan uint8
	c_closed chan uint8
	c_connect chan uint8
}

const (
	CLOSED = 0
	SYNRCVD = 1
	SYNSENT = 2
	EST = 3
	FINWAIT = 4
	TIMEWAIT = 5
	LASTACK = 6
)

func NewTunnel(remote *net.UDPAddr) (t *Tunnel, err error) {
	t = new(Tunnel)
	t.remote = remote
	t.status = CLOSED

	t.c_recv = make(chan []byte, 10)

	t.sendseq = 0
	t.recvseq = 0
	t.recvack = 0
	t.sendbuf = make([]*Packet, 0)
	t.recvbuf = make([]*Packet, 0)

	t.D = 200
	t.rtt = 200

	t.c_read = make(chan []byte, 10)
	t.c_write = make(chan []byte, 10)
	t.c_closing = make(chan uint8)
	t.c_closed = make(chan uint8)
	t.c_connect = make(chan uint8)

	go t.main()
	return
}

func DumpStatus(st uint8) string {
	switch st{
	case CLOSED: return "CLOSED"
	case SYNRCVD: return "SYNRCVD"
	case SYNSENT: return "SYNSENT"
	case EST: return "EST"
	case FINWAIT: return "FINWAIT"
	case TIMEWAIT: return "TIMEWAIT"
	case LASTACK: return "LASTACK"
	}
	return "unknown"
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
	for {
		select {
		case buf = <- t.c_recv: err = t.on_data(buf)
		case buf = <- t.c_write: err = t.send(0, buf)
		case <- t.c_closing:
			t.status = FINWAIT
			err = t.send(FIN, []byte{})
		}
		if err != nil { log.Println(err.Error()) }
	}
}

func (t *Tunnel) on_data(buf []byte) (err error) {
	var pkt *Packet
	var p *Packet

	if DEBUG { log.Println("recv in", t.Dump()) }
	pkt, err = Unpack(buf)
	if err != nil { return }

	if DEBUG { log.Println("recv packet", pkt.Dump()) }

	if (pkt.flag & ACK) != 0 {
		err = t.ack_recv(pkt)
		if err != nil { return }
	}

	switch{
	case (pkt.seq - t.recvseq) < 0:
		// return 
		return t.send(ACK, []byte{})
	case (pkt.seq - t.recvseq) == 0:
		for p = pkt; ; {
			err = t.proc_packet(p)
			if err != nil { return }

			if len(t.recvbuf) == 0 { break }
			if t.recvbuf[0].seq != t.recvseq { break }
			p = heap.Pop(&t.recvbuf).(*Packet)
		}
	case (pkt.seq - t.recvseq) > 0: t.recvbuf = append(t.recvbuf, pkt)
	}

	if t.recvseq != t.recvack {
		err = t.send(ACK, []byte{})
		if err != nil { return }
	}

	if DEBUG { log.Println("recv out", t.Dump()) }
	return
}

func (t *Tunnel) proc_packet(pkt *Packet) (err error) {

	if (pkt.flag & ACK) != 0 {
		if t.status == SYNRCVD {
			t.status = EST
			// most of time, this is useless
			// t.c_connect <- 1
		}
		if t.status == LASTACK {
			t.status = CLOSED
			// most of time, this is useless
			// t.c_close <- 1
			t.on_close()
		}
	}

	if (pkt.flag & SYN) != 0 {
		t.recvseq += 1
		if (pkt.flag & ACK) != 0 {
			if t.status != SYNSENT {
				return errors.New("status wrong, SYN ACK, " + t.Dump())
			}
			t.status = EST
			err = t.send(ACK, []byte{})
			if err != nil { return }
			t.c_connect <- 1
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

	if (pkt.flag & FIN) != 0 {
		t.recvseq += 1
		if (pkt.flag & ACK) != 0 {
			if t.status != FINWAIT {
				return errors.New("status wrong, FIN ACK, " + t.Dump())
			}
			t.status = TIMEWAIT
			err = t.send(ACK, []byte{})
			if err != nil { return }

			// wait 2*MSL to run close
			time.AfterFunc(time.Duration(2 * MSL) * time.Millisecond,
				func () { t.on_close() })

			t.c_closed <- 1
		}else{
			switch t.status {
			case EST:
				t.status = LASTACK
				err = t.send(FIN | ACK, []byte{})
				if err != nil { return }
			case FINWAIT:
				t.status = TIMEWAIT
				err = t.send(ACK, []byte{})
				if err != nil { return }
				
				// wait 2*MSL to run close
				time.AfterFunc(time.Duration(2 * MSL) * time.Millisecond,
					func () { t.on_close() })
			default:
				return errors.New("status wrong, FIN, " + t.Dump())
			}
		}
		return
	}

	if len(pkt.content) > 0 {
		t.recvseq += int32(len(pkt.content))
		t.c_read <- pkt.content
	}else if (pkt.flag & ^ACK) != 0 {
		t.recvseq += 1
	}

	return
}

func (t *Tunnel) ack_recv(pkt *Packet) (err error) {
	var sendbuf []*Packet
	var ti time.Time = time.Now()
	for _, p := range t.sendbuf {
		if p.seq >= pkt.ack {
			sendbuf = append(sendbuf, p)
		}else{
			M := int32(ti.Sub(p.t).Nanoseconds() / 1000000)
			t.D = uint32(0.875*float64(t.D) + 0.125*math.Abs(float64(int32(t.rtt)-M)))
			t.rtt = uint32(0.875*float64(t.rtt) + 0.125*float64(M))
			p.timeout.Stop()
		}
	}
	t.sendbuf = sendbuf
	return
}

func (t *Tunnel) send(flag uint8, content []byte) (err error) {
	if t.recvack != t.recvseq { flag |= ACK }

	err = t.send_packet(NewPacket(t, flag, content))
	if err != nil { return }
	t.recvack = t.recvseq

	if len(content) > 0 {
		t.sendseq += int32(len(content))
	}else if (flag & ^ACK) != 0 {
		t.sendseq += 1
	}

	if DEBUG { log.Println("send out", t.Dump()) }
	return
}

func (t *Tunnel) send_packet(pkt *Packet) (err error) {
	var buf []byte
	buf, err = pkt.Pack()
	if err != nil { return }

	if DEBUG { log.Println("send in", pkt.Dump()) }
	if DEBUG { log.Println("send", t.remote, buf) }

	t.c_send <- &DataBlock{t.remote, buf}

	if (pkt.flag & ^ACK) == 0 && len(pkt.content) == 0 { return }
	pkt.t = time.Now()
	t.sendbuf = append(t.sendbuf, pkt)

	d := time.Duration(t.rtt + 4*t.D) * time.Millisecond
	pkt.timeout = time.AfterFunc(d, func () {
		pkt.resend_count += 1
		if pkt.resend_count > MAXRESEND {
			log.Println("send packet more then maxresend times")
			t.on_close()
		}else{ t.send_packet(pkt) }
	})
	return
}

func (t *Tunnel) on_close () {
	if t.onclose != nil { t.onclose() }
}