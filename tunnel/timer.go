package tunnel

import (
	"time"
)

const (
	SLOWTICK = 5
	TM_TICK = 100
	TM_MSL = 30000
	TM_CONNEST = 75000
	TM_KEEPALIVE = 3600000
	TM_FINWAIT = 10000
	TM_PERSIST = 60000
)

type TcpTimer struct {
	ticker <-chan time.Time
	conn int32
	rexmt int32
	persist int32
	keep int32
	finwait int32
	timewait int32
	dack int32

	slow int8
}

func NewTimer () (tt *TcpTimer) {
	tt = new(TcpTimer)
	if OPT_DELAYACK {
		tt.ticker = time.Tick(TM_TICK * time.Millisecond)
	}else{
		tt.ticker = time.Tick(SLOWTICK * TM_TICK * time.Millisecond)
	}		
	tt.conn = TM_CONNEST
	tt.keep = TM_KEEPALIVE
	return
}

func (tt *TcpTimer) set_close () {
	tt.finwait = 0
	tt.timewait = 2*TM_MSL
}

func (tt *TcpTimer) on_timer (t *Tunnel) (err error) {
	if OPT_DELAYACK {
		tt.slow += 1
		if tt.slow >= SLOWTICK {
			tt.slow = 0
			err = tt.on_slow(t)
			if err != nil { panic(err) }
		}
		return tt.on_fast(t)
	}else{ return tt.on_slow(t) }
	return
}

func tick_timer(t int32) (int32, bool) {
	if t == 0 { return 0, false }
	next := t - TM_TICK
	if next <= 0 { return 0, true }
	return next, false
}

func (tt *TcpTimer) on_fast (t *Tunnel) (err error) {
	var trigger bool
	tt.dack, trigger = tick_timer(tt.dack)
	if trigger {
		t.logger.Debug("timer delayack")
		t.send(ACK, nil)
	}
	return
}

func (tt *TcpTimer) on_slow (t *Tunnel) (err error) {
	var trigger bool

	tt.conn, trigger = tick_timer(tt.conn)
	if trigger {
		t.logger.Debug("timer connest")
		t.drop()
	}

	tt.rexmt, trigger = tick_timer(tt.rexmt)
	if trigger {
		t.logger.Debug("timer retrans")
		if tt.rexmt != 0 { panic("persist timer not 0 when rexmt timer on") }
		err = t.on_retrans()
		// if err != nil { return }
		if err != nil { panic(err) }
	}

	tt.persist, trigger = tick_timer(tt.persist)
	if trigger {
		if tt.rexmt != 0 { panic("rexmt timer not 0 when persist timer on") }
		t.logger.Debug("timer persist")
		t.send(0, nil)
	}

	tt.keep, trigger = tick_timer(tt.keep)
	if trigger {
		t.logger.Debug("timer keepalive")
		t.drop()
	}

	tt.finwait, trigger = tick_timer(tt.finwait)
	if trigger {
		t.logger.Debug("timer finwait")
		t.drop()
	}

	tt.timewait, trigger = tick_timer(tt.timewait)
	if trigger {
		t.logger.Debug("timer timewait")
		t.c_event <- EV_END
	}

	return
}

func (t *Tunnel) on_retrans () (err error) {
	t.retrans_count += 1
	if t.retrans_count > MAXRESEND {
		t.drop()
		t.logger.Warning("send packet more then maxretrans times")
		return
	}

	for _, p := range t.sendbuf { t.send_packet(p) }

	inairlen := int32(0)
	if len(t.sendbuf) > 0 { inairlen = t.sendseq - t.sendbuf[0].seq }
	t.ssthresh = max32(int32(float32(inairlen)*BACKRATE), 2*SMSS)
	t.logger.Debug("congestion adjust, resend,", t.cwnd, t.ssthresh)

	t.timer.rexmt = t.rto * (1 << t.retrans_count)
	return
}
