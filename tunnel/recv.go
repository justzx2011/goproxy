package tunnel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

func (t *Tunnel) on_packet (pkt *Packet) (err error) {
	var p *Packet

	t.logger.Debug("recv", pkt)
	t.t_keep = TM_KEEPALIVE
	t.sendwnd = int32(pkt.window)

	if (pkt.flag & RST) != 0 {
		t.c_event <- EV_END
		return
	}
	if t.status == TIMEWAIT && (pkt.flag & SYN) != 0 {
		t.c_event <- EV_END
		return
	}

	if (pkt.flag & ACK) != 0 {
		ti := time.Now()

		for len(t.sendbuf) != 0 && (t.sendbuf[0].seq - pkt.ack) < 0 {
			p = t.sendbuf.Pop()

			delta := int32(ti.Sub(p.t).Nanoseconds() / 1000000) - int32(t.rtt)
			t.rtt = uint32(int32(t.rtt) + delta >> 3)
			t.rttvar = uint32(int32(t.rttvar) + (abs(delta) - int32(t.rttvar)) >> 2)

			put_packet(p)
		}

		switch {
		case t.sack_count >= 2 || t.retrans_count != 0:
			t.cwnd = t.ssthresh
		case t.cwnd <= t.ssthresh: t.cwnd += SMSS
		case t.cwnd < SMSS*SMSS: t.cwnd += SMSS*SMSS/t.cwnd
		default: t.cwnd += 1
		}
		t.logger.Debug("congestion adjust, ack,", t.cwnd, t.ssthresh)

		t.sack_count = 0
		t.retrans_count = 0
		if t.t_rexmt != 0 {
			if len(t.sendbuf) == 0 {
				t.t_rexmt = 0
			}else{
				t.t_rexmt = int32(t.rtt + t.rttvar << 2)
				t.t_rexmt -= int32(ti.Sub(t.sendbuf[0].t) / 1000000)
			}
		}
	}

	switch{
	case (pkt.seq - t.recvseq) < 0:
		put_packet(pkt)
		t.send(ACK, nil)
		return
	case (pkt.seq - t.recvseq) == 0:
		for p = pkt; ; {
			err = t.proc_current(p)
			put_packet(p)
			if err != nil { return }

			if len(t.recvbuf) == 0 { break }
			if t.recvbuf[0].seq != t.recvseq { break }
			p = t.recvbuf.Pop()
		}
	case (pkt.seq - t.recvseq) > 0:
		switch {
		case (pkt.flag & SACK) != 0:
			err = t.proc_sack(pkt)
			if err != nil { return }
			put_packet(pkt)
		case (len(pkt.content) > 0) || (pkt.flag != ACK):
			t.recvbuf.Push(pkt)
		default: put_packet(pkt)
		}
		err = t.send_sack()
		if err != nil { return }
	}

	if t.recvseq != t.recvack && t.t_dack == 0 {
		// t.t_dack = 1
		err = t.send(ACK, nil)
		if err != nil { return }
	}
	return
}

func (t *Tunnel) proc_current (pkt *Packet) (err error) {
	if t.status == SYNRCVD {
		t.t_conn = 0
		t.status = EST
	}
	if pkt.flag == ACK {
		if len(t.sendbuf) == 0 {
			switch t.status {
			case FINWAIT1:
				t.status = FINWAIT2
				t.send(FIN, nil)
			case CLOSING:
				t.status = TIMEWAIT
				t.t_finwait = 0
				t.t_2msl = 2*TM_MSL
				// t.t_2msl = int32(t.rtt << 3 + t.rttvar << 5)
				t.close_nowait()
			case LASTACK:
				t.status = CLOSED
				t.c_event <- EV_END
			}
		}
	}

	switch {
	case (pkt.flag & SACK) != 0: return t.proc_sack(pkt)
	case len(pkt.content) > 0:
		t.recvseq += int32(len(pkt.content))
		t.readlck.Lock()
		_, err = t.readbuf.Write(pkt.content)
		t.readlck.Unlock()
		if err != nil { return }
		select {
		case t.c_read <- 1:
		default:
		}
	case pkt.flag != ACK: t.recvseq += 1
	default: return
	}

	switch pkt.flag & ^ACK {
	case SYN:
		if (pkt.flag & ACK) != 0 {
			if t.status != SYNSENT {
				t.send(RST, nil)
				t.c_event <- EV_END
				return fmt.Errorf("SYN ACK status wrong, %s", t)
			}
			t.t_conn = 0
			t.status = EST
			err = t.send(ACK, nil)
			if err != nil { return }
			t.c_connect <- EV_CONNECTED
		}else{
			if t.status != CLOSED {
				t.send(RST, nil)
				t.c_event <- EV_END
				return fmt.Errorf("SYN status wrong, %s", t)
			}
			t.status = SYNRCVD
			err = t.send(SYN | ACK, nil)
		}
	case FIN:
		switch t.status {
		case TIMEWAIT: t.send(ACK, nil)
		case EST:
			t.status = LASTACK
			err = t.send(FIN | ACK, nil)
			t.c_write = nil
			return
		case FINWAIT1:
			if len(t.sendbuf) == 0 {
				t.status = TIMEWAIT
				err = t.send(ACK, nil)
				if err != nil { return }
				t.t_finwait = 0
				t.t_2msl = 2*TM_MSL
				// t.t_2msl = int32(t.rtt << 3 + t.rttvar << 5)
				t.close_nowait()
			}else{
				t.status = CLOSING
				err = t.send(ACK, nil)
			}
		case FINWAIT2:
			t.status = TIMEWAIT
			err = t.send(ACK, nil)
			if err != nil { return }
			t.t_finwait = 0
			t.t_2msl = 2*TM_MSL
			// t.t_2msl = int32(t.rtt << 3 + t.rttvar << 5)
			t.close_nowait()
		default:
			t.send(RST, nil)
			t.c_event <- EV_END
			return fmt.Errorf("FIN status wrong, %s", t)
		}
	}
	return
}

func (t *Tunnel) proc_sack(pkt *Packet) (err error) {
	var i int
	var id int32

	t.logger.Debug("sack proc")
	buf := bytes.NewBuffer(pkt.content)

	err = binary.Read(buf, binary.BigEndian, &id)
	switch err {
	case io.EOF:
	case nil:
		var sendbuf PacketQueue
		for i = 0; i < len(t.sendbuf); {
			p := t.sendbuf[i]
			df := p.seq - id
			switch {
			case df == 0:
				put_packet(t.sendbuf[i])
				i += 1
			case df < 0:
				sendbuf = append(sendbuf, t.sendbuf[i])
				i += 1
			}

			if df >= 0 {
				err = binary.Read(buf, binary.BigEndian, &id)
				switch err {
				case io.EOF:
					err = nil
					break
				case nil:
				default: return
				}
			}
		}
		if i < len(t.sendbuf) { sendbuf = append(sendbuf, t.sendbuf[i:]...) }
		t.sendbuf = sendbuf
	}

	t.sack_count += 1
	switch {
	case t.sack_count == RETRANS_SACKCOUNT:
		t.logger.Debug("first sack resend")

		inairlen := int32(0)
		if len(t.sendbuf) > 0 { inairlen = t.sendseq - t.sendbuf[0].seq }
		t.ssthresh = max32(inairlen >> 2, 2*SMSS)
		t.cwnd = t.ssthresh + 3*SMSS
		t.logger.Debug("congestion adjust, first sack,", t.cwnd, t.ssthresh)

		t.resend(id, true)
	case t.sack_count > RETRANS_SACKCOUNT:
		t.logger.Debug("sack resend")
		t.cwnd += SMSS
		t.logger.Debug("congestion adjust, sack,", t.cwnd, t.ssthresh)

		t.resend(id, true)
	}
	t.check_windows_block()

	return
}
