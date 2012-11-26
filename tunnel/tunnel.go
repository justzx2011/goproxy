package tunnel

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"../sutils"
)

type Tunnel struct {
	// status
	logger *sutils.Logger
	remote *net.UDPAddr
	status uint8
	stat Statistics
	timer *TcpTimer

	// communicate with conn loop
	c_recv chan *Packet // recv from network
	c_send chan *SendBlock // send to network
	onclose func ()

	// basic status
	seq_send int32
	seq_recv int32
	q_send PacketQueue
	q_recv PacketQueue

	// counter
	rtt uint32 // Round Trip Time
	rttvar uint32 // var of Round Trip Time
	rto int32 // Retransmission TimeOut
	recent int32 // 已经处理过的最后一个nettick
	sendwnd int32 // 接收方窗口
	cwnd int32 // 拥塞窗口
	ssthresh int32 // 拥塞窗口阀值
	sack_count uint // 当前连续接收了几个sack
	sack_sent map[int32]uint8 // 响应sack过程中，发送了哪些packet
	retrans_count uint // 当前连续发生了几次retransmission
	c_rexmt_in chan *Packet
	c_rexmt_out chan *Packet
	rexmt_idx int
	
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

func NewTunnel(remote *net.UDPAddr, name string, c_send chan *SendBlock) (t *Tunnel) {
	t = new(Tunnel)
	t.logger = sutils.NewLogger(name)
	t.remote = remote
	t.status = CLOSED
	t.timer = NewTimer()

	t.c_recv = make(chan *Packet, TBUFSIZE)
	t.c_send = c_send

	t.q_send = make(PacketQueue, 0)
	t.q_recv = make(PacketQueue, 0)

	t.rto = TM_INITRTO
	t.sendwnd = 10*MSS
	t.cwnd = 10*MSS
	t.ssthresh = WINDOWSIZE
	t.c_rexmt_in = make(chan *Packet, 3)
	t.rexmt_idx = -1

	t.c_read = make(chan uint8, 3)
	t.c_wrin = make(chan *Packet, 3)
	t.c_event = make(chan uint8, 3)
	t.c_connect = make(chan uint8)
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
		"st: %s, sseq: %d, rseq: %d, rcnt: %d, sbuf: %d, rbuf: %d, read: %d, write: %d, blk: %t",
		DumpStatus(t.status), t.seq_send, t.seq_recv, t.recent,
		len(t.q_send), len(t.q_recv), t.readbuf.Len(),
		len(t.c_wrin), t.c_wrout == nil)
}

func (t Tunnel) DumpCounter () string {
	return fmt.Sprintf(
		"rto: %d, cwnd: %d, ssth: %d, sack: %d, retrans: %d",
		t.rto, t.cwnd, t.ssthresh, t.sack_count, t.retrans_count)
}

func (t *Tunnel) main () {
	var err error
	var ev uint8
	var pkt *Packet
	defer t.on_quit()

QUIT:
	for {
		select {
		case ev = <- t.c_event:
			if ev == EV_END { break QUIT }
			t.logger.Debug("on event", ev)
			t.on_event(ev)
		case <- t.timer.ticker:
			t.timer.on_timer(t)
			continue
		case pkt = <- t.c_recv:
			err = pkt.Unpack()
			if err != nil {
				t.logger.Err(err)
				continue
			}
			err = t.on_packet(pkt)
			if err != nil { panic(err) }
		case pkt = <- t.c_wrout:
			t.send(0, pkt)
		case pkt = <- t.c_rexmt_out:
			t.send_packet(pkt)
			switch {
			case t.rexmt_idx < 0:
				t.c_rexmt_out = nil
			case t.rexmt_idx >= t.q_send.Len():
				t.rexmt_idx = -1
				t.c_rexmt_out = nil
			default:
				t.c_rexmt_in <- t.q_send.Get(t.rexmt_idx)
				t.rexmt_idx += 1
			}
		} // TODO: split read and write
		t.timer.chk_rexmt(t)
		t.check_windows_block()
		t.logger.Debug("loop", t)
	}
}

func (t *Tunnel) check_windows_block () {
	if t.rexmt_idx != -1 {
		inairlen := int32(0)
		switch {
		case t.q_send.Len() == 0:
		case t.rexmt_idx == t.q_send.Len():
			inairlen = t.seq_send - t.q_send.Front().seq
		case t.q_send.Len() > 0:
			inairlen = t.q_send.Get(t.rexmt_idx).seq - t.q_send.Front().seq
		}
		switch {
		case inairlen >= t.cwnd:
			if t.c_rexmt_out != nil {
				t.logger.Info("blocking,", inairlen, t.sendwnd, t.cwnd, t.ssthresh)
				t.c_rexmt_out = nil
			}
		case t.c_rexmt_out == nil && t.status == EST:
			t.logger.Info("restart,", inairlen, t.sendwnd, t.cwnd, t.ssthresh)
			t.c_rexmt_out = t.c_rexmt_in
		}
	}else{
		inairlen := int32(0)
		if t.q_send.Len() > 0 { inairlen = t.seq_send - t.q_send.Front().seq }
		switch {
		case inairlen >= t.sendwnd:
			if t.c_wrout != nil {
				t.logger.Info("blocking,", inairlen, t.sendwnd, t.cwnd, t.ssthresh)
				t.c_wrout = nil
				t.timer.persist = TM_PERSIST
			}
		case inairlen >= t.cwnd:
			if t.c_wrout != nil {
				t.logger.Info("blocking,", inairlen, t.sendwnd, t.cwnd, t.ssthresh)
				t.c_wrout = nil
			}
		case t.c_wrout == nil && t.status == EST:
			t.logger.Info("restart,", inairlen, t.sendwnd, t.cwnd, t.ssthresh)
			t.c_wrout = t.c_wrin
		}
	}
}

func (t *Tunnel) on_event (ev uint8) {
	switch ev {
	case EV_CONNECT:
		if t.status != CLOSED {
			t.drop("somebody try to connect, " + t.String())
			return
		}
		t.status = SYNSENT
		t.send(SYN, nil)
	case EV_CLOSE:
		if t.status != EST { return }
		t.c_wrout = nil
		t.status = FINWAIT1
		t.send(FIN, nil)
		t.timer.finwait = TM_FINWAIT
	case EV_READ: t.send(ACK, nil)
	default:
		t.logger.Err("unknown event", ev)
		t.c_event <- EV_END
	}
	return
}

func (t *Tunnel) on_quit () {
	t.logger.Info("quit")
	t.logger.Info(t.DumpCounter())
	t.logger.Info(t.stat.String())

	t.status = CLOSED
	close(t.c_read)
	close(t.c_wrin)
	if t.onclose != nil { t.onclose() }

	for _, p := range t.q_send { put_packet(p) }
	for _, p := range t.q_recv { put_packet(p) }
}

func (t *Tunnel) isquit () (bool) {
	select {
	case <- t.c_close: return true
	default:
	}
	return false
}

func (t *Tunnel) drop (s string) {
	t.send(RST, nil)
	t.c_event <- EV_END
	t.logger.Info(s)
}
