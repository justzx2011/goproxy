package tunnel

import (
	"bytes"
	"encoding/binary"
	"io"
	"time"
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

	if t.status == TIMEWAIT {
		switch pkt.flag & ACKMASK {
		case SYN:
			t.c_event <- EV_END
			return
		case FIN:
			t.send(ACK, nil)
			return
		}
	}

	diff := pkt.seq - t.recvseq
	timediff := pkt.sndtime - t.recent
	// TODO: accelerate for recv normal data

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
		}else if !t.recvbuf.Push(pkt) {
			put_packet(pkt)
		}
		t.send_sack()
		return
	}

	// 提取历史数据
	sendack := false
	for p = pkt; ; p = t.recvbuf.Pop() {
		if t.proc_current(p) { sendack = true }
		put_packet(p)

		if len(t.recvbuf) == 0 || (t.recvbuf.Front().seq != t.recvseq) {
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
	t.readlck.Lock()
	defer t.readlck.Unlock()
	size := len(pkt.content)
	_, err := t.readbuf.Write(pkt.content)
	if err != nil { panic(err) }
	select {
	case t.c_read <- 1:
	default:
	}
	t.recvseq += int32(size)
	t.stat.recvsize += uint64(size)
}

func (t *Tunnel) proc_current (pkt *Packet) (sendack bool) {
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
			t.recv_data(pkt)
			return true
		}
		if pkt.flag == DAT { t.logger.Warning("DAT packet without data") }
		// ack in this way
	case PST:
		t.recvseq += 1
		t.send(ACK, nil)
		return
	default: t.recvseq += 1
	}// ack with no data and all flags other then pst, rst and sack

	switch t.status {
	case EST:
		if flag == FIN {
			t.status = LASTACK
			t.send(FIN | ACK, nil)
			t.c_wrout = nil
			return
		}
	case FINWAIT1:
		switch flag {
		case FIN:
			if (pkt.flag & ACK) != 0 && (pkt.ack == t.sendseq) {
				t.status = TIMEWAIT
				t.send(ACK, nil)
				t.timer.set_close()
				close(t.c_close)
				return
			}else{
				t.status = CLOSING
				t.send(ACK, nil)
				return
			}
		case ACK:
			if pkt.ack == t.sendseq {
				t.status = FINWAIT2
				t.send(FIN, nil)
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
		if pkt.flag == ACK && pkt.ack == t.sendseq {
			t.status = TIMEWAIT
			t.timer.set_close()
			close(t.c_close)
			return
		}
	case LASTACK:
		if pkt.flag == ACK && pkt.ack == t.sendseq {
			t.status = CLOSED
			t.c_event <- EV_END
			close(t.c_close)
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
	case CLOSED:
		if pkt.flag != SYN {
			t.drop("CLOSED got a no SYN, " + DumpFlag(pkt.flag))
			return
		}
		t.status = SYNRCVD
		t.send(SYN | ACK, nil)
		return
	}

	if flag != DAT {
		t.logger.Warning("unknown flag or not processed,", pkt)
	}
	return
}

func (t *Tunnel) recv_ack(pkt *Packet) {
	var p *Packet
	ntick := int32(time.Now().UnixNano()/NETTICK)

	for len(t.sendbuf) != 0 && (t.sendbuf.Front().seq - pkt.ack) < 0 {
		p = t.sendbuf.Pop()

		t.stat.sendpkt += 1
		t.stat.sendsize += uint64(len(p.content))
		t.stat.senderr -= 1

		put_packet(p)
	}

	if t.rtt == 0 {
		t.rtt = uint32(ntick - pkt.acktime)
		t.rttvar = t.rtt / 2
	}else{
		delta := ntick - pkt.acktime - int32(t.rtt)
		t.rtt = uint32(int32(t.rtt) + delta >> 3)
		t.rttvar = uint32(int32(t.rttvar) + (abs(delta) - int32(t.rttvar)) >> 2)
	}
	t.rto = int32((t.rtt + t.rttvar << 2)/NETTICK_M)
	// if t.rto < TM_TICK { t.rto = TM_TICK }
	t.logger.Info("rtt info,", t.rtt, t.rttvar, t.rto)

	resend_flag := t.sack_count >= 2 || t.retrans_count != 0
	switch {
	case resend_flag: t.cwnd = t.ssthresh
	case t.cwnd <= t.ssthresh: t.cwnd += MSS
	case t.cwnd < MSS*MSS: t.cwnd += MSS*MSS/t.cwnd
	default: t.cwnd += 1
	}
	t.sack_count = 0
	t.sack_sent = nil
	t.retrans_count = 0
	t.logger.Info("congestion adjust, ack,", t.cwnd, t.ssthresh)

	if t.timer.rexmt != 0 {
		if len(t.sendbuf) != 0 {
			t.timer.rexmt = t.rto
		}else{ t.timer.rexmt = 0 }
	}
	return 
}

func (t *Tunnel) recv_sack(pkt *Packet) {
	var i int
	var id int32
	var err error

	t.logger.Debug("sack proc", t.sendbuf)
	t.stat.recverr += 1
	buf := bytes.NewBuffer(pkt.content)

	err = binary.Read(buf, binary.BigEndian, &id)
	// t.logger.Debug("sack id", id)
	switch err {
	case io.EOF: err = nil
	case nil:
		var sendbuf PacketQueue
LOOP: // sendbuf...
		for i = 0; i < len(t.sendbuf); {
			p := t.sendbuf[i]
			df := p.seq - id
			switch {
			case df == 0:
				put_packet(p)
				i += 1
			case df < 0:
				sendbuf = append(sendbuf, p)
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
				// t.logger.Debug("sack id", id)
			}
		}
		if i < len(t.sendbuf) { sendbuf = append(sendbuf, t.sendbuf[i:]...) }
		t.sendbuf = sendbuf
	default: panic(err)
	}
	t.logger.Debug("sack proc end", t.sendbuf)

	t.sack_count += 1
	switch {
	case t.sack_count < RETRANS_SACKCOUNT: return
	case t.sack_count == RETRANS_SACKCOUNT:
		inairlen := int32(0)
		if len(t.sendbuf) > 0 { inairlen = t.sendseq - t.sendbuf.Front().seq }
		t.ssthresh = max32(inairlen/2, 2*MSS)
		t.cwnd = t.ssthresh + 3*MSS
		t.logger.Info("congestion adjust, first sack,", t.cwnd, t.ssthresh)
	case t.sack_count > RETRANS_SACKCOUNT:
		t.cwnd += MSS
		t.logger.Info("congestion adjust, sack,", t.cwnd, t.ssthresh)
	}

	var ok bool
	if t.sack_sent == nil { t.sack_sent = make(map[int32]uint8) }
	for _, p := range t.sendbuf {
		if (p.seq - id) >= 0 { break }
		if t.sack_sent != nil {
			_, ok = t.sack_sent[p.seq]
			if ok { continue }
		}
		t.send_packet(p)
		t.sack_sent[p.seq] = 1
	}
	t.timer.rexmt = t.rto * (1 << t.retrans_count)
	t.logger.Debug("reset rexmt due to sack", t.timer.rexmt)
	return
}