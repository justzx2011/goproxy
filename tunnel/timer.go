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

const (
	NETTICK = 1000 * 100 // nanosecond
	NETTICK_S = 1000 * 1000 * 1000 / NETTICK
	NETTICK_M = 1000 * 1000 / NETTICK
)

type TcpTimer struct {
	ticker <-chan time.Time
	conn int32
	rexmt int32
	persist int32
	keep int32
	finwait int32
	timewait int32

	slow int8
}

func NewTimer () (tt *TcpTimer) {
	tt = new(TcpTimer)
	tt.ticker = time.Tick(TM_TICK * time.Millisecond)
	tt.conn = TM_CONNEST
	tt.keep = TM_KEEPALIVE
	return
}

func (tt *TcpTimer) set_close () {
	tt.finwait = 0
	tt.timewait = 2*TM_MSL
}

func (tt *TcpTimer) on_timer (t *Tunnel) {
	tt.slow += 1
	if tt.slow >= SLOWTICK {
		tt.slow = 0
		tt.on_slow(t)
	}
	tt.on_fast(t)
}

func tick_timer(t int32) (int32, bool) {
	if t == 0 { return 0, false }
	next := t - TM_TICK
	if next <= 0 { return 0, true }
	return next, false
}

func (tt *TcpTimer) on_fast (t *Tunnel) {
	var trigger bool

	rexmtbak := tt.rexmt
	tt.rexmt, trigger = tick_timer(tt.rexmt)
	if trigger {
		t.logger.Info("timer retrans", rexmtbak)
		if tt.rexmt != 0 { panic("persist timer not 0 when rexmt timer on") }
		t.on_retrans()
	}
	return
}

func (tt *TcpTimer) on_slow (t *Tunnel) {
	var trigger bool

	tt.conn, trigger = tick_timer(tt.conn)
	if trigger {
		t.drop("connect timeout")
	}

	tt.persist, trigger = tick_timer(tt.persist)
	if trigger {
		if tt.rexmt != 0 { panic("rexmt timer not 0 when persist timer on") }
		t.logger.Debug("timer persist")
		t.send(PST, nil)
	}

	tt.keep, trigger = tick_timer(tt.keep)
	if trigger {
		t.drop("keepalive timeout")
	}

	tt.finwait, trigger = tick_timer(tt.finwait)
	if trigger {
		t.drop("fin wait timeout")
	}

	tt.timewait, trigger = tick_timer(tt.timewait)
	if trigger {
		t.logger.Debug("timer timewait")
		t.c_event <- EV_END
	}

	return
}