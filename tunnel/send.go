package tunnel

import (
	"bytes"
	"encoding/binary"
	"math/rand"
)

func (t *Tunnel) send_sack () (err error) {
	t.logger.Debug("sack send", t.q_recv)
	pkt := get_packet()
	buf := bytes.NewBuffer(pkt.buf[HEADERSIZE:HEADERSIZE])
	for i, p := range t.q_recv {
		if i >= MSS/4 - 1 { break }
		binary.Write(buf, binary.BigEndian, p.seq)
	}
	pkt.content = buf.Bytes()
	t.send(SACK, pkt)
	return
}

func (t *Tunnel) send (flag uint8, pkt *Packet) {
	if t.status != EST && flag == DAT { panic(flag) }

	if pkt == nil {
		pkt = get_packet()
		pkt.content = pkt.buf[HEADERSIZE:HEADERSIZE]
	}
	pkt.read_status(t, flag)
	size := len(pkt.content)
	if size != 0 && (flag & ACKMASK) != DAT && (flag & ACKMASK) != SACK{
		panic(pkt)
	}

	t.send_packet(pkt)

	// 不参与seq递增的包，也不需要retrans
	switch flag & ACKMASK {
	case SACK: return
	case DAT:
		if size == 0 { return }
		t.seq_send += int32(size)
	default: t.seq_send += 1
	}
	// TODO: 不加入q_send的pkt的回收
	// 不能直接回收，会导致发送时有问题

	if !t.q_send.Push(pkt) { panic(pkt) }
	// FIXME: if rto < TM_TICK?
	if t.timer.rexmt_work == 0 && t.rto != 0 {
		t.logger.Debug("set rexmt,", t.rto)
		t.timer.rexmt = get_nettick() + t.rto
		t.timer.rexmt_work = 1
	}
	return
}

func (t *Tunnel) send_packet(pkt *Packet) {
	t.stat.senderr += 1
	pkt.sndtime = get_nettick()
	pkt.acktime = t.recent
	t.logger.Debug("send", pkt)

	if DROPFLAG && rand.Intn(100) >= DROPRATE {
		t.logger.Debug("drop packet")
	}else{
		err := pkt.Pack()
		if err != nil { panic(err) }
		t.c_send <- &SendBlock{t.remote, pkt}
	}
	return
}

func (t *Tunnel) on_retrans () {
	if t.q_send.Len() == 0 { return }

	t.retrans_count += 1
	if t.retrans_count > MAXRESEND {
		t.drop("send packet more then maxretrans times")
		return
	}
	
	t.c_rexmt_in <- t.q_send.Get(0)
	t.rexmt_idx = 1
	t.c_wrout = nil
	t.c_rexmt_out = t.c_rexmt_in
	
	t.sack_count = 0
	t.sack_sent = nil

	inairlen := int32(0)
	if len(t.q_send) > 0 { inairlen = t.seq_send - t.q_send.Front().seq }
	t.ssthresh = max32(inairlen/2, 2*MSS)
	t.cwnd = MSS
	t.logger.Info("congestion adjust, resend,", t.cwnd, t.ssthresh)

	t.timer.rexmt = get_nettick() + t.rto * (1 << t.retrans_count)
	t.timer.rexmt_work = 1
	return
}
