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
		t.c_event <- EV_END
		put_packet(pkt)
		return
	}

	diff := (pkt.seq - t.recvseq)
	size := len(pkt.content)
	// TODO: PAWS
	// accelerate for recv normal data
	if t.status == EST && pkt.flag == DAT && diff == 0 && size != 0 &&
		(len(t.recvbuf) == 0 || t.recvbuf.Front().seq != pkt.seq + int32(size)) {
		t.logger.Debug("fast data recv")
		t.recv_data(pkt)

		p = get_packet()
		p.content = p.buf[HEADERSIZE:HEADERSIZE]
		p.acktime = pkt.sndtime
		t.send(ACK, p)

		put_packet(pkt)
		return
	}

	if diff < 0 {
		// or same in timer, older in seq
		if (pkt.flag & ACKMASK) == DAT { t.send(ACK, nil) }
		put_packet(pkt)
		// when diff < 0, return now
		return
	}

	if (pkt.flag & ACK) != 0 {
		t.recv_ack(pkt)
	} // do ack and sack first
	if (pkt.flag & ACKMASK) == SACK {
		t.recv_sack(pkt)
		put_packet(pkt)
		return
	}

	if diff > 0 {
		if size == 0 && pkt.flag == ACK {
			put_packet(pkt)
		}else if !t.recvbuf.Push(pkt) {
			put_packet(pkt)
		}
		t.send_sack()
		return
	}

	// 提取历史数据
	sendack := false
	var sndtime int64
	var cnt int64
	for p = pkt; ; p = t.recvbuf.Pop() {
		if t.proc_current(p) { sendack = true }

		sndtime += int64(p.sndtime)
		cnt += 1
		t.logger.Debug(sndtime, cnt, p.sndtime)
		put_packet(p)

		if len(t.recvbuf) == 0 || (t.recvbuf.Front().seq != t.recvseq) {
			break
		}
	}

	if sendack {
		select {
		case p = <- t.c_wrout:
		default: p = nil
		}

		if p == nil {
			p = get_packet()
			p.content = p.buf[HEADERSIZE:HEADERSIZE]
		}
		p.acktime = int32(sndtime/cnt)
		t.logger.Debug(p.acktime, cnt, p.sndtime)
		t.send(ACK, p)
	}

	return
}

func (t *Tunnel) recv_data (pkt *Packet) {
	t.readlck.Lock()
	defer t.readlck.Unlock()
	_, err := t.readbuf.Write(pkt.content)
	if err != nil { panic(err) }
	select {
	case t.c_read <- 1:
	default:
	}
	size := len(pkt.content)
	t.recent = pkt.sndtime
	t.recvseq += int32(size)
	t.stat.recvsize += uint64(size)
}

func (t *Tunnel) proc_current (pkt *Packet) (sendack bool) {
	t.sendwnd = int32(pkt.window)

	flag := pkt.flag & ACKMASK
	if flag == DAT && len(pkt.content) != 0 {
		t.recv_data(pkt)
		return true
	}

	switch t.status {
	case EST:
		if flag == FIN {
			t.recvseq += 1
			t.status = LASTACK
			t.send(FIN | ACK, nil)
			t.c_wrout = nil
			return
		}
	case TIMEWAIT:
		switch flag {
		case SYN:
			t.recvseq += 1
			t.c_event <- EV_END
			return
		case FIN:
			t.recvseq += 1
			t.send(ACK, nil)
			return
		}
	case FINWAIT1:
		switch pkt.flag {
		case FIN:
			if (pkt.flag & ACK) != 0 && (pkt.ack == t.sendseq) {
				t.recvseq += 1
				t.status = TIMEWAIT
				t.send(ACK, nil)
				t.timer.set_close()
				t.close_nowait()
				return
			}else{
				t.recvseq += 1
				t.status = CLOSING
				t.send(ACK, nil)
				return
			}
		case ACK:
			if pkt.ack == t.sendseq {
				t.recvseq += 1
				t.status = FINWAIT2
				t.send(FIN, nil)
				return
			}
		}
	case FINWAIT2:
		if flag == FIN {
			t.recvseq += 1
			t.status = TIMEWAIT
			t.send(ACK, nil)
			t.timer.set_close()
			t.close_nowait()
			return
		}
	case CLOSING:
		if pkt.flag == ACK && pkt.ack == t.sendseq {
			t.recvseq += 1
			t.status = TIMEWAIT
			t.timer.set_close()
			t.close_nowait()
			return
		}
	case LASTACK:
		if pkt.flag == ACK && pkt.ack == t.sendseq {
			t.recvseq += 1
			t.status = CLOSED
			t.c_event <- EV_END
			return
		}
	case SYNRCVD:
		if (pkt.flag & ACKMASK) == DAT {
			t.recvseq += 1
			t.timer.conn = 0
			t.status = EST
			t.c_wrout = t.c_wrin
			return
		}
	case SYNSENT:
		if pkt.flag != SYN | ACK {
			t.logger.Warning("SYNSENT got a no SYN ACK, ", DumpFlag(pkt.flag))
			return
		}
		t.recvseq += 1
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
		t.recvseq += 1
		t.status = SYNRCVD
		t.send(SYN | ACK, nil)
		return
	}

	switch flag {
	case DAT:
		if pkt.flag == DAT {
			t.logger.Warning("data packet with no data")
		}
	case SYN: t.logger.Warning("SYN flag not processed,", t)
	case FIN: t.logger.Warning("FIN flag not processed,", t)
	case PST: t.send(ACK, nil)
	default: t.logger.Warning("what the hell flag is? ", pkt)
	}
	return
}

func (t *Tunnel) recv_ack(pkt *Packet) {
	var p *Packet
	ntick := get_nettick()

	for len(t.sendbuf) != 0 && (t.sendbuf.Front().seq - pkt.ack) < 0 {
		p = t.sendbuf.Pop()

		t.stat.sendpkt += 1
		t.stat.sendsize += uint64(len(p.content))
		t.stat.senderr -= 1

		put_packet(p)
	}

	if pkt.acktime != 0 {
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
	}

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
	t.logger.Debug("sack id", id)
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
				t.logger.Debug("sack id", id)
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
		t.ssthresh = max32(int32(float32(inairlen)*BACKRATE), 2*MSS)
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
	return
}