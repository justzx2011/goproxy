package tunnel

import (
	"errors"
	"fmt"
	"net"
	"time"
	"../sutils"
)

type Tunnel struct {
	// status
	logger *sutils.Logger
	remote *net.UDPAddr
	status uint8

	// communicate with conn loop
	c_recv chan []byte
	c_send chan *DataBlock
	onclose func ()

	// basic status
	sendseq int32
	recvseq int32
	recvack int32
	sendbuf PacketQueue
	recvbuf PacketQueue
	sendwnd uint32
	recvwnd uint32

	// counter
	rtt uint32
	rttvar uint32
	sack_count uint
	retrans_count uint

	// timer
	connest <-chan time.Time
	retrans <-chan time.Time
	delayack <-chan time.Time
	keepalive <-chan time.Time
	finwait <-chan time.Time
	timewait <-chan time.Time

	// communicate with conn
	c_read chan []byte
	c_write chan []byte
	c_wrbak chan []byte
	c_event chan uint8
	c_connect chan uint8
	c_close chan uint8
}

func NewTunnel(remote *net.UDPAddr, name string) (t *Tunnel) {
	t = new(Tunnel)
	t.logger = sutils.NewLogger(name)
	t.remote = remote
	t.status = CLOSED

	t.c_recv = make(chan []byte, 1)

	t.sendseq = 0
	t.recvseq = 0
	t.recvack = 0
	t.sendbuf = make(PacketQueue, 0)
	t.recvbuf = make(PacketQueue, 0)
	t.sendwnd = 0
	t.recvwnd = WINDOWSIZE

	t.rtt = 200000
	t.rttvar = 200000
	t.sack_count = 0
	t.retrans_count = 0
	t.keepalive = time.After(time.Duration(TM_KEEPALIVE) * time.Second)

	t.c_read = make(chan []byte, 64 * 1024)
	t.c_write = make(chan []byte, 1)
	t.c_wrbak = t.c_write
	t.c_event = make(chan uint8, 1)
	t.c_connect = make(chan uint8, 1)
	t.c_close = make(chan uint8, 3)

	go t.main()
	return
}

func (t Tunnel) Dump() string {
	return fmt.Sprintf(
		"st: %s, sseq: %d, rseq: %d, sbuf: %d, rbuf: %d, read: %d, write: %d",
		DumpStatus(t.status), t.sendseq, t.recvseq,
		len(t.sendbuf), len(t.recvbuf), len(t.c_read), len(t.c_wrbak))
}

func (t *Tunnel) main () {
	var err error
	var buf []byte
	var ev uint8

	defer func () {
		t.logger.Info("quit")
		t.status = CLOSED
		close(t.c_read)
		for len(t.c_close) < 2 { t.c_close <- EV_CLOSED }
		close(t.c_wrbak)
		if t.onclose != nil { t.onclose() }
	}()

QUIT:
	for {
		select {
		case ev = <- t.c_event:
			if ev == EV_END { break QUIT }
			t.logger.Debug("on event", ev)
			err = t.on_event(ev)
		case <- t.connest:
			t.logger.Debug("timer connest")
			t.send(RST, []byte{})
			t.c_event <- EV_END
		case <- t.retrans:
			t.logger.Debug("timer retrans")
			err = t.on_retrans()
		case <- t.delayack:
			t.logger.Debug("timer delayack")
			err = t.send(ACK, []byte{})
		case <- t.keepalive:
			t.logger.Debug("timer keepalive")
			t.send(RST, []byte{})
			t.c_event <- EV_END
		case <- t.finwait:
			t.logger.Debug("timer finwait")
			t.send(RST, []byte{})
			t.c_event <- EV_END
		case <- t.timewait:
			t.logger.Debug("timer timewait")
			t.c_event <- EV_END
		case buf = <- t.c_recv: err = t.on_data(buf)
		case buf = <- t.c_write: err = t.send(0, buf)
		}
		if err != nil { t.logger.Err(err) }
		t.logger.Debug(t.Dump())
	}
}

func (t *Tunnel) on_event (ev uint8) (err error) {
	switch ev {
	case EV_CONNECT:
		if t.status != CLOSED {
			t.send(RST, []byte{})
			t.c_event <- EV_END
			return errors.New("somebody try to connect, " + t.Dump())
		}
		t.connest = time.After(time.Duration(TM_CONNEST) * time.Second)
		t.status = SYNSENT
		return t.send(SYN, []byte{})
	case EV_CLOSE:
		if t.status != EST { return }
		t.finwait = time.After(time.Duration(TM_FINWAIT) * time.Millisecond)
		t.status = FINWAIT1
		t.c_write = nil
		return t.send(FIN, []byte{})
	}
	return errors.New("unknown event")
}

func (t *Tunnel) on_data(buf []byte) (err error) {
	var next bool
	var pkt *Packet
	var p *Packet

	pkt, err = Unpack(buf)
	if err != nil { return }

	t.logger.Debug("recv", pkt.Dump())
	t.keepalive = time.After(time.Duration(TM_KEEPALIVE) * time.Second)

	next, err = t.proc_now(pkt)
	if err != nil { return err }
	if !next { return }

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