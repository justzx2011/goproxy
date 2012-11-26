package tunnel

import (
	"bytes"
	"encoding/binary"
	"io"
)

func (t *Tunnel) on_packet (pkt *Packet) (err error) {
	var p *Packet

	t.logger.Debug("recv", pkt)
	t.stat.recvpkt += 1
	t.timer.keep = TM_KEEPALIVE

	// no reset packet next
	if (pkt.flag & ACKMASK) == RST {
		t.logger.Debug("recv RST")
		t.c_event <- EV_END
		put_packet(pkt)
		return
	}

	switch t.status {
	case TIMEWAIT:
		switch pkt.flag & ACKMASK {
		case SYN:
			t.c_event <- EV_END
			return
		case FIN:
			t.send(ACK, nil)
			return
		}
	}

	diff := pkt.seq - t.seq_recv
	timediff := pkt.sndtime - t.recent

	if (t.recent != 0 && timediff < 0) || diff < 0 {
		// or same in timer, older in seq
		if (t.recent != 0 && timediff < 0) {
			t.logger.Debug("history packet by timer")
		}else{ t.logger.Debug("history packet") }
		if (pkt.flag & ACKMASK) == DAT { t.send(ACK, nil) }
		put_packet(pkt)
		// when diff < 0, return now
		return
	}
	if diff <= 0 { t.recent = pkt.sndtime }

	if (pkt.flag & ACK) != 0 {
		t.logger.Debug("recv ack")
		t.recv_ack(pkt)
	} // do ack and sack first
	if (pkt.flag & ACKMASK) == SACK {
		t.logger.Debug("recv sack")
		t.recv_sack(pkt)
		put_packet(pkt)
		// future sack mean some packet loss
		if diff > 0 { t.send_sack() }
		return
	}

	if diff > 0 {
		t.logger.Debug("future packet")
		if len(pkt.content) == 0 && pkt.flag == ACK {
			put_packet(pkt)
		}else if !t.q_recv.Push(pkt) {
			put_packet(pkt)
		}
		t.send_sack()
		return
	}

	// 提取历史数据
	sendack := false
	for p = pkt; ; p = t.q_recv.Pop() {
		sa, bk := t.proc_current(p)
		if bk { return }
		if sa { sendack = true }
		put_packet(p)

		if len(t.q_recv) == 0 || (t.q_recv.Front().seq != t.seq_recv) {
			break
		}
	}

	if sendack {
		t.logger.Debug("send ack back")
		select {
		case p = <- t.c_wrout:
		default: p = nil
		}
		t.send(ACK, p)
	}

	return
}

func (t *Tunnel) recv_data (pkt *Packet) {
}

func (t *Tunnel) proc_current (pkt *Packet) (sendack bool, bk bool) {
	t.sendwnd = int32(pkt.window)
	t.logger.Debug("process current,", pkt.seq)

	flag := pkt.flag & ACKMASK
	if t.status == SYNRCVD && flag == DAT {
		t.timer.conn = 0
		t.status = EST
		t.c_wrout = t.c_wrin
	}

	switch flag {
	case DAT:
		if len(pkt.content) != 0 {
			t.readlck.Lock()
			defer t.readlck.Unlock()
			size := len(pkt.content)
			_, err := t.readbuf.Write(pkt.content)
			if err != nil { panic(err) }
			select {
			case t.c_read <- 1:
			default:
			}
			t.seq_recv += int32(size)
			t.stat.recvsize += uint64(size)
			return true, false
		}
		if pkt.flag == DAT { t.logger.Warning("DAT packet without data") }
		// ack in this way
	case PST:
		t.seq_recv += 1
		t.send(ACK, nil)
		return
	default: t.seq_recv += 1
	}// ack with no data and all flags other then pst, rst and sack

	switch t.status {
	case EST:
		if flag == FIN {
			t.c_wrout = nil
			t.status = LASTACK
			t.send(FIN | ACK, nil)
			return
		}
	case FINWAIT1:
		switch flag {
		case FIN:
			if (pkt.flag & ACK) != 0 && (pkt.ack == t.seq_send) {
				t.status = TIMEWAIT
				t.send(ACK, nil)
				t.timer.set_close()
				close(t.c_close)
			}else{
				t.status = CLOSING
				t.send(ACK, nil)
			}
			return
		case ACK:
			if pkt.ack == t.seq_send {
				t.status = FINWAIT2
				return
			}
		}
	case FINWAIT2:
		if flag == FIN {
			t.status = TIMEWAIT
			t.send(ACK, nil)
			t.timer.set_close()
			close(t.c_close)
			return
		}
	case CLOSING:
		if pkt.flag == ACK && pkt.ack == t.seq_send {
			t.status = TIMEWAIT
			t.timer.set_close()
			close(t.c_close)
			return
		}
	case LASTACK:
		if pkt.flag == ACK && pkt.ack == t.seq_send {
			t.status = CLOSED
			t.c_event <- EV_END
			close(t.c_close)
			return
		}
	case CLOSED:
		if pkt.flag != SYN {
			t.drop("CLOSED got a no SYN, " + DumpFlag(pkt.flag))
			return
		}
		t.status = SYNRCVD
		t.send(SYN | ACK, nil)
		return
	case SYNRCVD:
		if flag == FIN {
			t.c_wrout = nil
			t.status = LASTACK
			t.send(FIN | ACK, nil)
			return
		}
	case SYNSENT:
		if pkt.flag != SYN | ACK {
			t.logger.Warning("SYNSENT got a no SYN ACK, ", DumpFlag(pkt.flag))
			return
		}
		t.timer.conn = 0
		t.status = EST
		t.send(ACK, nil)
		t.c_wrout = t.c_wrin
		t.c_connect <- EV_CONNECTED
		return
	}

	if flag != DAT {
		t.drop("unknown flag or not processed, " + pkt.String() + DumpStatus(t.status))
		return false, true
	}
	return
}

func (t *Tunnel) recv_ack(pkt *Packet) {
	var p *Packet
	ntick := get_nettick()
	
	for len(t.q_send) != 0 && (t.q_send.Front().seq - pkt.ack) < 0 {
		p = t.q_send.Pop()

		t.stat.sendpkt += 1
		t.stat.sendsize += uint64(len(p.content))
		t.stat.senderr -= 1
		
		// if rexmt_idx == 0, set new idx to 0, resend continue form un-acked area
		switch t.rexmt_idx {
		case -1, 0:
		default: t.rexmt_idx -= 1
		}

		put_packet(p)
	}
	// if len(q_send) == 0, that make rexmt_idx = 0, and rexmt_idx == q_send.Len()
	// so it will quit resend mode in next resend

	if t.rtt == 0 {
		t.rtt = uint32(ntick - pkt.acktime)
		t.rttvar = t.rtt / 2
	}else{
		delta := ntick - pkt.acktime - int32(t.rtt)
		t.rtt = uint32(int32(t.rtt) + delta >> 3)
		t.rttvar = uint32(int32(t.rttvar) + (abs(delta) - int32(t.rttvar)) >> 2)
	}
	t.rto = int32(t.rtt + t.rttvar << 2)
	t.logger.Info("rtt info,", t.rtt, t.rttvar, t.rto)

	switch {
	case t.sack_count >= 2: t.cwnd = t.ssthresh
	case t.cwnd <= t.ssthresh: t.cwnd += MSS
	case t.cwnd < MSS*MSS: t.cwnd += MSS*MSS/t.cwnd
	default: t.cwnd += 1
	}
	t.sack_count = 0
	t.sack_sent = nil
	t.retrans_count = 0
	t.logger.Info("congestion adjust, ack,", t.cwnd, t.ssthresh)

	if t.timer.rexmt != 0 {
		if len(t.q_send) != 0 {
			t.timer.rexmt = get_nettick() + t.rto
			t.timer.rexmt_work = 1
		}else{ t.timer.rexmt_work = 0 }
	}
	return 
}

func (t *Tunnel) recv_sack(pkt *Packet) {
	var i int
	var id int32
	var err error

	t.logger.Debug("sack proc", t.q_send)
	t.stat.recverr += 1
	buf := bytes.NewBuffer(pkt.content)

	err = binary.Read(buf, binary.BigEndian, &id)
	switch err {
	case io.EOF: err = nil
	case nil:
		var q_send PacketQueue
LOOP: // q_send...
		for i = 0; i < len(t.q_send); {
			p := t.q_send[i]
			df := p.seq - id
			switch {
			case df == 0:
				switch t.rexmt_idx {
				case -1, 0:
				default:
					if i < t.rexmt_idx { t.rexmt_idx -= 1 }
				}

				put_packet(p)
				i += 1
			case df < 0:
				q_send = append(q_send, p)
				i += 1
			}

			if df >= 0 {
				err = binary.Read(buf, binary.BigEndian, &id)
				switch err {
				case io.EOF:
					err = nil
					break LOOP
				case nil:
				default: panic(err)
				}
			}
		}
		if i < len(t.q_send) { q_send = append(q_send, t.q_send[i:]...) }
		t.q_send = q_send
	default: panic(err)
	}
	t.logger.Debug("sack proc end", t.q_send)

	if t.q_send.Len() == 0 { return }
	t.sack_count += 1

	switch {
	case t.sack_count < RETRANS_SACKCOUNT: return
	case t.sack_count == RETRANS_SACKCOUNT:
		inairlen := int32(0)
		if len(t.q_send) > 0 { inairlen = t.seq_send - t.q_send.Front().seq }
		t.ssthresh = max32(inairlen, 2*MSS)
		t.cwnd = 10*MSS
		t.logger.Info("congestion adjust, first sack,", t.cwnd, t.ssthresh)
	case t.sack_count > RETRANS_SACKCOUNT:
		t.cwnd += MSS
		t.logger.Info("congestion adjust, sack,", t.cwnd, t.ssthresh)
	}

	if t.rexmt_idx == -1 {
		t.c_rexmt_in <- t.q_send.Get(0)
		t.rexmt_idx = 1
		t.c_wrout = nil
		t.c_rexmt_out = t.c_rexmt_in
	}

	t.timer.rexmt = get_nettick() + t.rto * (1 << t.retrans_count)
	t.timer.rexmt_work = 1
	t.logger.Debug("reset rexmt due to sack", t.timer.rexmt)
	return
}