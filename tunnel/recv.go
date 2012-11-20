package tunnel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

func (t *Tunnel) on_packet (pkt *Packet) (err error) {
	var next bool
	var p *Packet

	t.logger.Debug("recv", pkt)
	t.keepalive = time.After(time.Duration(TM_KEEPALIVE) * time.Second)

	next, err = t.proc_now(pkt)
	if err != nil { return err }
	if !next { return }

	switch{
	case (pkt.seq - t.recvseq) < 0:
		put_packet(pkt)
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
		t.recvbuf.Push(pkt)
		err = t.send_sack()
	}

	if t.recvseq != t.recvack && t.delayack == nil {
		t.delayack = time.After(time.Duration(TM_DELAYACK) * time.Millisecond)
	}

	return
}

func (t *Tunnel) proc_now (pkt *Packet) (next bool, err error) {
	if (pkt.flag & RST) != 0 {
		t.c_event <- EV_END
		return false, err
	}
	if t.status == TIMEWAIT && (pkt.flag & SYN) != 0 {
		t.c_event <- EV_END
		return false, err
	}
	if (pkt.flag & ACK) != 0 {
		err = t.ack_recv(pkt)
		if err != nil { return }
	}

	t.sendwnd = int32(pkt.window)
	t.check_windows_block()
	return true, nil
}

func (t *Tunnel) ack_recv(pkt *Packet) (err error) {
	var ti time.Time = time.Now()
	var p *Packet

	for len(t.sendbuf) != 0 && t.sendbuf[0].seq < pkt.ack {
		p = t.sendbuf.Pop()

		delta := int32(ti.Sub(p.t).Nanoseconds() / 1000000) - int32(t.rtt)
		t.rtt = uint32(int32(t.rtt) + delta >> 3)
		t.rttvar = uint32(int32(t.rttvar) + (abs(delta) - int32(t.rttvar)) >> 2)

		put_packet(p)
	}

	switch {
	case t.sack_count != 0 || t.retrans_count != 0:
		t.cwnd = t.ssthresh
	case t.cwnd <= t.ssthresh: t.cwnd += SMSS
	case t.cwnd < SMSS*SMSS: t.cwnd += SMSS*SMSS/t.cwnd
	default: t.cwnd += 1
	}

	t.retrans_count = 0
	if t.retrans != nil {
		if len(t.sendbuf) == 0 {
			t.retrans = nil
		}else{
			d := time.Duration(t.rtt + t.rttvar << 2) 
			d -= ti.Sub(t.sendbuf[0].t)
			t.retrans = time.After(d * time.Millisecond)
		}
	}

	t.sack_count = 0
	return
}

func (t *Tunnel) proc_current (pkt *Packet) (err error) {
	if t.status == SYNRCVD { t.status = EST }
	if pkt.flag == ACK {
		err = t.proc_ack(pkt)
		if err != nil { return }
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
	case (pkt.flag != ACK): t.recvseq += 1
	default: return
	}

	switch pkt.flag & ^ACK {
	case SYN: return t.proc_syn(pkt)
	case FIN: return t.proc_fin(pkt)
	}
	return
}

func (t *Tunnel) proc_ack (pkt *Packet) (err error) {
	if len(t.sendbuf) == 0 {
		switch t.status {
		case FINWAIT1:
			t.status = FINWAIT2
			t.send(FIN, nil)
		case CLOSING:
			t.status = TIMEWAIT
			t.finwait = nil
			// t.timewait = time.After(2*time.Duration(TM_MSL)*time.Millisecond)
			t.timewait = time.After(time.Duration(t.rtt << 3 + t.rttvar << 5) * time.Millisecond)
			for len(t.c_close) < 2 { t.c_close <- EV_CLOSED }
		case LASTACK:
			t.status = CLOSED
			t.c_event <- EV_END
		}
	}
	return
}

func (t *Tunnel) filter_sendbuf (buf *bytes.Buffer) (sendbuf PacketQueue, err error) {
	var i int
	var id int32

	err = binary.Read(buf, binary.BigEndian, &id)
	if err == io.EOF { return t.sendbuf, nil }
	if err != nil { return }

	for i = 0; i < len(t.sendbuf); {
		p := t.sendbuf[i]
		df := p.seq - id
		switch {
		case df == 0:
			// t.logger.Notice("hit id", id)
			put_packet(t.sendbuf[i])
			i += 1
		case df < 0:
			sendbuf = append(sendbuf, t.sendbuf[i])
			i += 1
		}

		if df >= 0 {
			err = binary.Read(buf, binary.BigEndian, &id)
			if err == io.EOF {
				err = nil
				break
			}
			if err != nil { return }
		}
	}
	if i < len(t.sendbuf) { sendbuf = append(sendbuf, t.sendbuf[i:]...) }
	return
}

func (t *Tunnel) proc_sack(pkt *Packet) (err error) {
	var id int32
	t.logger.Warning("sack proc", t.sendbuf.String())
	buf := bytes.NewBuffer(pkt.content)

	sendbuf, err := t.filter_sendbuf(buf)
	if err != nil { return }
	t.sendbuf = sendbuf

	t.sack_count += 1
	switch {
	case t.sack_count == RETRANS_SACKCOUNT:
		t.logger.Warning("first sack resend")

		inairlen := int32(0)
		if len(t.sendbuf) > 0 { inairlen = t.sendseq - t.sendbuf[0].seq }
		t.ssthresh = max32(inairlen/2, 2*SMSS)

		t.resend(id, true)
		t.cwnd = t.ssthresh + 3*SMSS
	case t.sack_count > RETRANS_SACKCOUNT:
		t.logger.Warning("sack resend")

		t.resend(id, true)
		t.cwnd += SMSS
	}
	t.check_windows_block()

	return
}

func (t *Tunnel) proc_syn (pkt *Packet) (err error) {
	if (pkt.flag & ACK) != 0 {
		if t.status != SYNSENT {
			t.send(RST, nil)
			t.c_event <- EV_END
			return fmt.Errorf("SYN ACK status wrong, %s", t)
		}
		t.connest = nil
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
	return
}

func (t *Tunnel) proc_fin (pkt *Packet) (err error) {
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
			t.finwait = nil
			// t.timewait = time.After(2*time.Duration(TM_MSL)*time.Millisecond)
			t.timewait = time.After(time.Duration(t.rtt << 3 + t.rttvar << 5) * time.Millisecond)
			for len(t.c_close) < 2 { t.c_close <- EV_CLOSED }
		}else{
			t.status = CLOSING
			err = t.send(ACK, nil)
		}
	case FINWAIT2:
		t.status = TIMEWAIT
		err = t.send(ACK, nil)
		if err != nil { return }
		t.finwait = nil
		// t.timewait = time.After(2*time.Duration(TM_MSL)*time.Millisecond)
		t.timewait = time.After(time.Duration(t.rtt << 3 + t.rttvar << 5) * time.Millisecond)
		for len(t.c_close) < 2 { t.c_close <- EV_CLOSED }
	default:
		t.send(RST, nil)
		t.c_event <- EV_END
		return fmt.Errorf("FIN status wrong, %s", t)
	}
	return
}