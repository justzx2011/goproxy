package tunnel

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"runtime"
	"sync"
	"time"
	"../sutils"
)

// TODO: 持续定时器

type Tunnel struct {
	// status
	logger *sutils.Logger
	remote *net.UDPAddr
	status uint8

	// communicate with conn loop
	c_recv chan *Packet
	c_send chan *SendBlock
	onclose func ()

	// basic status
	sendseq int32
	recvseq int32
	recvack int32
	sendbuf PacketQueue
	recvbuf PacketQueue
	sendwnd int32

	// counter
	rtt uint32
	rttvar uint32
	cwnd int32
	ssthresh int32
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
	readlck sync.Mutex
	readbuf bytes.Buffer
	c_read chan uint8
	// readlen int32
	c_write chan *Packet
	c_wrbak chan *Packet
	c_event chan uint8
	c_connect chan uint8
	c_close chan uint8
}

func NewTunnel(remote *net.UDPAddr, name string) (t *Tunnel) {
	t = new(Tunnel)
	t.logger = sutils.NewLogger(name)
	t.remote = remote
	t.status = CLOSED

	t.c_recv = make(chan *Packet, 1)

	t.sendseq = 0
	t.recvseq = 0
	t.recvack = 0
	t.sendbuf = make(PacketQueue, 0)
	t.recvbuf = make(PacketQueue, 0)
	t.sendwnd = 4*SMSS

	t.rtt = 200000
	t.rttvar = 200000
	t.cwnd = int32(min(4*SMSS, max(2*SMSS, 4380)))
	t.ssthresh = WINDOWSIZE
	t.sack_count = 0
	t.retrans_count = 0
	t.keepalive = time.After(time.Duration(TM_KEEPALIVE) * time.Second)

	t.c_read = make(chan uint8)
	t.c_write = make(chan *Packet, 1)
	t.c_wrbak = t.c_write
	t.c_event = make(chan uint8, 1)
	t.c_connect = make(chan uint8, 1)
	t.c_close = make(chan uint8, 3)

	go t.main()
	return
}

func (t Tunnel) String () string {
	return "st: " + DumpStatus(t.status)
}

func (t *Tunnel) Dump() string {
	return fmt.Sprintf(
		"st: %s, sseq: %d, rseq: %d, sbuf: %d, rbuf: %d, read: %d, write: %d, blk: %t",
		DumpStatus(t.status), t.sendseq, t.recvseq,
		len(t.sendbuf), len(t.recvbuf), t.readbuf.Len(),
		len(t.c_wrbak), t.c_write == nil)
}

func (t Tunnel) DumpCounter () string {
	return fmt.Sprintf(
		"rtt: %d, var: %d, cwnd: %d, ssth: %d, sack: %d, retrans: %d",
		t.rtt, t.rttvar, t.cwnd, t.ssthresh, t.sack_count, t.retrans_count)
}

func (t *Tunnel) main () {
	var err error
	var ev uint8
	var pkt *Packet
	// var rb *RecvBlock
	defer t.on_quit()

QUIT:
	for {
		select {
		case ev = <- t.c_event:
			if ev == EV_END { break QUIT }
			t.logger.Debug("on event", ev)
			err = t.on_event(ev)
		case <- t.connest:
			t.logger.Debug("timer connest")
			t.send(RST, nil)
			t.c_event <- EV_END
		case <- t.retrans:
			t.logger.Debug("timer retrans")
			err = t.on_retrans()
		case <- t.delayack:
			t.logger.Debug("timer delayack")
			err = t.send(ACK, nil)
		case <- t.keepalive:
			t.logger.Debug("timer keepalive")
			t.send(RST, nil)
			t.c_event <- EV_END
		case <- t.finwait:
			t.logger.Debug("timer finwait")
			t.send(RST, nil)
			t.c_event <- EV_END
		case <- t.timewait:
			t.logger.Debug("timer timewait")
			t.c_event <- EV_END
		case pkt = <- t.c_recv: err = t.on_packet(pkt)
		case pkt = <- t.c_write: err = t.send(0, pkt)
		}
		if err != nil { t.logger.Err(err) }
		t.logger.Debug(t.Dump())
		// t.logger.Debug(t.DumpCounter())
	}
}

func (t *Tunnel) on_event (ev uint8) (err error) {
	switch ev {
	case EV_CONNECT:
		if t.status != CLOSED {
			t.send(RST, nil)
			t.c_event <- EV_END
			return fmt.Errorf("somebody try to connect, %s", t)
		}
		t.connest = time.After(time.Duration(TM_CONNEST) * time.Second)
		t.status = SYNSENT
		return t.send(SYN, nil)
	case EV_CLOSE:
		if t.status != EST { return }
		t.finwait = time.After(time.Duration(TM_FINWAIT) * time.Millisecond)
		t.status = FINWAIT1
		t.c_write = nil
		return t.send(FIN, nil)
	case EV_READ:
		return t.send(ACK, nil)
	}
	return errors.New("unknown event")
}

func (t *Tunnel) on_quit () {
	t.logger.Info("quit")
	t.logger.Info(t.DumpCounter())
	t.status = CLOSED
	close(t.c_read)
	for len(t.c_close) < 2 { t.c_close <- EV_CLOSED }
	close(t.c_wrbak)
	if t.onclose != nil { t.onclose() }
	if rand.Intn(100) > 95 {
		runtime.GC()
	}
}
