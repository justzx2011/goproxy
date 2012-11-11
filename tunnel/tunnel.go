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

const MSL = 60

// timeout when closing

type Tunnel struct {
	conn *net.UDPConn
	remote *net.UDPAddr
	status uint8

	sendseq int32
	recvseq int32
	sendbuf PacketHeap
	recvbuf PacketHeap
	recvack int32
	recvcha chan uint8
	buf *bytes.Buffer

	D uint32
	rtt uint32
	onclose func ()
	c_connect chan uint8
	c_close chan uint8
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

func NewTunnel(conn *net.UDPConn, remote *net.UDPAddr) (t *Tunnel, err error) {
	t = new(Tunnel)
	t.conn = conn
	t.remote = remote
	t.status = CLOSED

	t.sendseq = 0
	t.recvseq = 0
	t.sendbuf = make([]*Packet, 0)
	t.recvbuf = make([]*Packet, 0)
	t.recvack = 0
	t.recvcha = make(chan uint8, 10)
	t.buf = bytes.NewBuffer([]byte{})

	t.D = 200
	t.rtt = 200
	t.c_connect = make(chan uint8)
	t.c_close = make(chan uint8)
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
		"status: %s, sendseq: %d, recvseq: %d, recvack: %d, sendbuf: %d, recvbuf: %d, buf: %d",
		DumpStatus(t.status), t.sendseq, t.recvseq, t.recvack,
		len(t.sendbuf), len(t.recvbuf), t.buf.Len())
	return buf.String()
}

func (t *Tunnel) OnData(buf []byte) (err error) {
	var pkt *Packet
	var p *Packet

	log.Println()
	log.Println("recv in", t.Dump())
	pkt, err = Unpack(buf)
	if err != nil { return }

	log.Println("recv packet", pkt.Dump())

	if (pkt.flag & ACK) != 0 {
		err = t.ackRecv(pkt)
		if err != nil { return }
	}

	switch{
	case (pkt.seq - t.recvseq) < 0:
		// return 
		return t.send(ACK, []byte{})
	case (pkt.seq - t.recvseq) == 0:
		for p = pkt; ; {
			err = t.procPacket(p)
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
	if t.buf.Len() > 0 && len(t.recvcha) == 0 { t.recvcha <- 1 }

	log.Println("recv out", t.Dump())
	log.Println()
	return
}

func (t *Tunnel) procPacket(pkt *Packet) (err error) {

	log.Println("proc packet")

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
			if t.onclose != nil { t.onclose() }
		}
	}

	if (pkt.flag & SYN) != 0 {
		t.recvseq += 1
		if (pkt.flag & ACK) != 0 {
			if t.status != SYNSENT { return errors.New("status wrong") }
			t.status = EST
			err = t.send(ACK, []byte{})
			if err != nil { return }
			t.c_connect <- 1
		}else{
			if t.status != CLOSED { return errors.New("status wrong") }
			t.status = SYNRCVD
			err = t.send(SYN | ACK, []byte{})
			if err != nil { return }
		}
		return
	}

	if (pkt.flag & FIN) != 0 {
		t.recvseq += 1
		if (pkt.flag & ACK) != 0 {
			if t.status != FINWAIT { return errors.New("status wrong") }
			t.status = TIMEWAIT
			err = t.send(ACK, []byte{})
			if err != nil { return }

			// wait 2*MSL to run t.onclose()
			if t.onclose != nil {
				time.AfterFunc(time.Duration(2 * MSL) * time.Second,
					t.onclose)
			}

			t.c_close <- 1
		}else{
			if t.status != EST { return errors.New("status wrong") }
			t.status = LASTACK
			err = t.send(FIN | ACK, []byte{})
			if err != nil { return }
		}
		return
	}

	if len(pkt.content) > 0 {
		var n int
		t.recvseq += int32(len(pkt.content))
		n, err = t.buf.Write(pkt.content)
		if err != nil { return }
		if n != len(pkt.content) {
			return errors.New("recv buffer full")
		}
	}else if (pkt.flag & ^ACK) != 0 {
		t.recvseq += 1
	}

	return
}

func (t *Tunnel) ackRecv(pkt *Packet) (err error) {
	// filter sendbuf for bi.seq < pkt.seq
	// FIXME: should I lock?
	// TODO: math M and renew RTT
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

	err = t.sendPacket(NewPacket(t, flag, content))
	if err != nil { return }
	t.recvack = t.recvseq

	if len(content) > 0 {
		t.sendseq += int32(len(content))
	}else if (flag & ^ACK) != 0 {
		t.sendseq += 1
	}

	log.Println("send out", t.Dump())
	return
}

func (t *Tunnel) sendPacket(pkt *Packet) (err error) {
	var buf []byte
	buf, err = pkt.Pack()
	if err != nil { return }

	var n int
	log.Println("send in", pkt.Dump())
	log.Println("send", t.remote, buf)
	if t.remote == nil {
		n, err = t.conn.Write(buf)
	}else{
		n, err = t.conn.WriteToUDP(buf, t.remote)
	}
	if err != nil { return }
	if n != len(buf) { return errors.New("send buffer full") }

	if (pkt.flag & ^ACK) == 0 && len(pkt.content) == 0 { return }
	pkt.t = time.Now()
	t.sendbuf = append(t.sendbuf, pkt)

	// FIXME: RTT is not like that
	d := time.Duration(t.rtt + 4*t.D) * time.Millisecond
	pkt.timeout = time.AfterFunc(d, func () {
		pkt.resend_count += 1
		if pkt.resend_count > MAXRESEND {
			log.Println("send packet more then maxresend times")
			if t.onclose != nil { t.onclose() }
		}else{ t.sendPacket(pkt) }
	})
	return
}
