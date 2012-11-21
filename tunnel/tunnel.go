package tunnel

import (
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"runtime"
	"sync"
	"time"
	"../sutils"
)

type Tunnel struct {
	// status
	logger *sutils.Logger
	remote *net.UDPAddr
	status uint8
	stat Statistics

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
	ticker <-chan time.Time
	t_conn int32
	t_rexmt int32
	t_persist int32
	t_keep int32
	t_finwait int32
	t_2msl int32
	t_dack int32
	
	// communicate with conn
	readlck sync.Mutex
	readbuf bytes.Buffer
	c_read chan uint8
	c_wrin chan *Packet
	c_wrout chan *Packet
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

	t.sendbuf = make(PacketQueue, 0)
	t.recvbuf = make(PacketQueue, 0)
	t.sendwnd = 4*SMSS

	t.rtt = 200
	t.rttvar = 200
	t.cwnd = int32(min(4*SMSS, max(2*SMSS, 4380)))
	t.ssthresh = WINDOWSIZE

	t.ticker = time.Tick(TM_TICK * time.Millisecond)
	t.t_conn = TM_CONNEST
	t.t_keep = TM_KEEPALIVE

	t.c_read = make(chan uint8)
	t.c_wrin = make(chan *Packet, 3)
	t.c_wrout = t.c_wrin
	t.c_event = make(chan uint8, 3)
	t.c_connect = make(chan uint8, 1)
	t.c_close = make(chan uint8)

	go t.main()
	return
}

func (t Tunnel) String () string {
	// return "st: " + DumpStatus(t.status)
	return t.Dump()
}

func (t *Tunnel) Dump() string {
	return fmt.Sprintf(
		"st: %s, sseq: %d, rseq: %d, sbuf: %d, rbuf: %d, read: %d, write: %d, blk: %t",
		DumpStatus(t.status), t.sendseq, t.recvseq,
		len(t.sendbuf), len(t.recvbuf), t.readbuf.Len(),
		len(t.c_wrin), t.c_wrout == nil)
}

func (t Tunnel) DumpCounter () string {
	return fmt.Sprintf(
		"rtt: %d, var: %d, cwnd: %d, ssth: %d, sack: %d, retrans: %d",
		t.rtt, t.rttvar, t.cwnd, t.ssthresh, t.sack_count, t.retrans_count)
}

func (t *Tunnel) main () {
	var err error
	var recycly bool
	var ev uint8
	var pkt *Packet
	defer t.on_quit()

QUIT:
	for {
		select {
		case ev = <- t.c_event:
			if ev == EV_END { break QUIT }
			t.logger.Debug("on event", ev)
			err = t.on_event(ev)
		case <- t.ticker:
			err = t.on_timer()
			// if err != nil { t.logger.Err(err) }
			if err != nil { panic(err) }
			continue
		case pkt = <- t.c_recv:
			recycly, err = t.on_packet(pkt)
			if recycly { put_packet(pkt) }
			t.check_windows_block()
		case pkt = <- t.c_wrout:
			err = t.send(0, pkt)
			t.check_windows_block()
		}
		// if err != nil { t.logger.Err(err) }
		if err != nil { panic(err) }
		t.logger.Debug("loop", t.Dump())
		t.logger.Debug("stat", t.stat.String())
	}
}

func (t *Tunnel) check_windows_block () {
	inairlen := int32(0)
	if len(t.sendbuf) > 0 { inairlen = t.sendseq - t.sendbuf[0].seq }
	switch {
	case (inairlen >= t.sendwnd) || (inairlen >= t.cwnd):
		t.logger.Debug("blocking,", inairlen, t.sendwnd, t.cwnd, t.ssthresh)
		t.c_wrout = nil
	case t.status == EST && t.c_wrout == nil:
		t.logger.Debug("restart,", inairlen, t.sendwnd, t.cwnd, t.ssthresh)
		t.c_wrout = t.c_wrin
	}
}

func (t *Tunnel) on_event (ev uint8) (err error) {
	switch ev {
	case EV_CONNECT:
		if t.status != CLOSED {
			err = t.send(RST, nil)
			if err != nil { panic(err) }
			t.c_event <- EV_END
			return fmt.Errorf("somebody try to connect, %s", t)
		}
		t.status = SYNSENT
		err = t.send(SYN, nil)
		if err != nil { panic(err) }
	case EV_CLOSE:
		if t.status != EST { return }
		t.t_finwait = TM_FINWAIT
		t.status = FINWAIT1
		t.c_wrout = nil
		err = t.send(FIN, nil)
		if err != nil { panic(err) }
	case EV_READ:
		err = t.send(ACK, nil)
		if err != nil { panic(err) }
	default: return fmt.Errorf("unknown event %d", ev)
	}
	return
}

func tick_timer(t int32) (int32, bool) {
	if t == 0 { return 0, false }
	next := t - TM_TICK
	if next <= 0 { return 0, true }
	return next, false
}

func (t *Tunnel) on_timer () (err error) {
	var trigger bool

	t.t_conn, trigger = tick_timer(t.t_conn)
	if trigger {
		t.logger.Debug("timer connest")
		t.c_event <- EV_END
		err = t.send(RST, nil)
		// if err != nil { return }
		if err != nil { panic(err) }
	}

	t.t_rexmt, trigger = tick_timer(t.t_rexmt)
	if trigger {
		t.logger.Debug("timer retrans")
		err = t.on_retrans()
		// if err != nil { return }
		if err != nil { panic(err) }
	}

	t.t_persist, trigger = tick_timer(t.t_persist)
	if trigger {
		t.logger.Debug("timer persist")
		// TODO: 持续定时器
		// err = t.on_retrans()
	}

	t.t_keep, trigger = tick_timer(t.t_keep)
	if trigger {
		t.logger.Debug("timer keepalive")
		t.c_event <- EV_END
		err = t.send(RST, nil)
		// if err != nil { return }
		if err != nil { panic(err) }
	}

	t.t_finwait, trigger = tick_timer(t.t_finwait)
	if trigger {
		t.logger.Debug("timer finwait")
		t.c_event <- EV_END
		err = t.send(RST, nil)
		// if err != nil { return }
		if err != nil { panic(err) }
	}

	t.t_2msl, trigger = tick_timer(t.t_2msl)
	if trigger {
		t.logger.Debug("timer timewait")
		t.c_event <- EV_END
	}

	t.t_dack, trigger = tick_timer(t.t_dack)
	if trigger {
		t.logger.Debug("timer delayack")
		err = t.send(ACK, nil)
		// if err != nil { return }
		if err != nil { panic(err) }
	}
	return
}

func (t *Tunnel) on_quit () {
	t.logger.Info("quit")
	t.logger.Info(t.DumpCounter())

	t.status = CLOSED
	t.close_nowait()
	
	close(t.c_read)
	close(t.c_wrin)
	if t.onclose != nil { t.onclose() }

	if rand.Intn(100) > 95 { runtime.GC() }
	for _, p := range t.sendbuf { put_packet(p) }
	for _, p := range t.recvbuf { put_packet(p) }
}

func (t *Tunnel) close_nowait () {
	select {
	case <- t.c_close:
	default: close(t.c_close)
	}
}

func (t *Tunnel) isquit () (bool) {
	select {
	case <- t.c_close: return true
	default:
	}
	return false
}