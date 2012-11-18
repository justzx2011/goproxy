package tunnel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

func (t *Tunnel) on_data (buf []byte) (err error) {
	var next bool
	var pkt *Packet
	var p *Packet

	pkt, err = Unpack(buf)
	if err != nil { return }

	t.logger.Debug("recv", pkt)
	t.keepalive = time.After(time.Duration(TM_KEEPALIVE) * time.Second)

	next, err = t.proc_now(pkt)
	if err != nil { return err }
	if !next { return }

	switch{
	case (pkt.seq - t.recvseq) < 0: return 
	case (pkt.seq - t.recvseq) == 0:
		for p = pkt; ; {
			err = t.proc_packet(p)
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

		delta := int32(ti.Sub(p.t).Nanoseconds() / 1000) - int32(t.rtt)
		t.rtt = uint32(int32(t.rtt) + delta >> 3)
		t.rttvar = uint32(int32(t.rttvar) + (abs(delta) - int32(t.rttvar)) >> 2)
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
			t.retrans = time.After(d * time.Microsecond)
		}
	}

	t.sack_count = 0
	return
}

func (t *Tunnel) proc_packet (pkt *Packet) (err error) {
	// var rb *RecvBlock

	if t.status == SYNRCVD { t.status = EST }
	if pkt.flag == ACK {
		err = t.proc_ack(pkt)
		if err != nil { return }
	}

	switch {
	case (pkt.flag & SACK) != 0: return t.proc_sack(pkt)
	case len(pkt.content) > 0:
		t.recvseq += int32(len(pkt.content))
		// rb = get_recvblock()
		t.c_read <- pkt.content
		atomic.AddInt32(&t.readlen, int32(len(pkt.content)))
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
			t.send(FIN, []byte{})
		case CLOSING:
			t.status = TIMEWAIT
			t.finwait = nil
			// t.timewait = time.After(2*time.Duration(TM_MSL)*time.Millisecond)
			t.timewait = time.After(time.Duration(t.rtt << 3 + t.rttvar << 5))
			for len(t.c_close) < 2 { t.c_close <- EV_CLOSED }
		case LASTACK:
			t.status = CLOSED
			t.c_event <- EV_END
		}
	}
	return
}

func (t *Tunnel) proc_sack(pkt *Packet) (err error) {
	var id int32
	var sendbuf PacketQueue
	t.logger.Warning("sack proc")
	buf := bytes.NewBuffer(pkt.content)

	binary.Read(buf, binary.BigEndian, &id)
	for _, p := range t.sendbuf {
		if p.seq == id {
			err = binary.Read(buf, binary.BigEndian, &id)
			if err == io.EOF {
				err = nil
				break
			}
			if err != nil { return }
		}else{ sendbuf = append(sendbuf, p) }
	}
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
			t.send(RST, []byte{})
			t.c_event <- EV_END
			return fmt.Errorf("SYN ACK status wrong, %s", t)
		}
		t.connest = nil
		t.status = EST
		err = t.send(ACK, []byte{})
		if err != nil { return }
		t.c_connect <- EV_CONNECTED
	}else{
		if t.status != CLOSED {
			t.send(RST, []byte{})
			t.c_event <- EV_END
			return fmt.Errorf("SYN status wrong, %s", t)
		}
		t.status = SYNRCVD
		err = t.send(SYN | ACK, []byte{})
	}
	return
}

func (t *Tunnel) proc_fin (pkt *Packet) (err error) {
	switch t.status {
	case TIMEWAIT: t.send(ACK, []byte{})
	case EST:
		t.status = LASTACK
		err = t.send(FIN | ACK, []byte{})
		t.c_write = nil
		return
	case FINWAIT1:
		if len(t.sendbuf) == 0 {
			t.status = TIMEWAIT
			err = t.send(ACK, []byte{})
			if err != nil { return }
			t.finwait = nil
			// t.timewait = time.After(2*time.Duration(TM_MSL)*time.Millisecond)
			t.timewait = time.After(time.Duration(t.rtt << 3 + t.rttvar << 5))
			for len(t.c_close) < 2 { t.c_close <- EV_CLOSED }
		}else{
			t.status = CLOSING
			err = t.send(ACK, []byte{})
		}
	case FINWAIT2:
		t.status = TIMEWAIT
		err = t.send(ACK, []byte{})
		if err != nil { return }
		t.finwait = nil
		// t.timewait = time.After(2*time.Duration(TM_MSL)*time.Millisecond)
		t.timewait = time.After(time.Duration(t.rtt << 3 + t.rttvar << 5))
		for len(t.c_close) < 2 { t.c_close <- EV_CLOSED }
	default:
		t.send(RST, []byte{})
		t.c_event <- EV_END
		return fmt.Errorf("FIN status wrong, %s", t)
	}
	return
}