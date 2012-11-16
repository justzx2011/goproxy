package tunnel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/rand"
	"time"
)

func (t *Tunnel) send_sack () (err error) {
	t.logger.Warning("sack send")
	buf := bytes.NewBuffer([]byte{})
	for i, p := range t.recvbuf {
		if i > 0x7f { break }
		binary.Write(buf, binary.BigEndian, p.seq)
	}
	return t.send(SACK, buf.Bytes())
}

func (t *Tunnel) send (flag uint8, content []byte) (err error) {
	if t.status != EST && flag == 0{
		return errors.New("can't send data, " + DumpFlag(flag) + ", " + t.Dump())
	}

	if t.recvack != t.recvseq { flag |= ACK }
	err = t.send_packet(NewPacket(t, flag, content), true)
	if err != nil { return }

	if int(t.sendwnd) < len(content) {
		t.sendwnd = 0
	}else{ t.sendwnd -= uint32(len(content)) }
	if t.sendwnd == 0 {
		t.c_write = nil
	}

	switch {
	case (flag & SACK) != 0:
	case len(content) > 0: t.sendseq += int32(len(content))
	case flag != ACK: t.sendseq += 1
	}

	t.recvack = t.recvseq
	if t.delayack != nil { t.delayack = nil }
	return
}

func (t *Tunnel) send_packet(pkt *Packet, retrans bool) (err error) {
	var buf []byte
	t.logger.Debug("send", pkt.Dump())

	buf, err = pkt.Pack()
	if err != nil { return }

	if DROPFLAG && rand.Intn(100) >= 85 {
		t.logger.Debug("drop packet")
	}else{
		t.c_send <- &DataBlock{t.remote, buf}
	}
	if (pkt.flag & SACK) != 0 { return }
	if pkt.flag == ACK && len(pkt.content) == 0 { return }
	if !retrans { return }

	pkt.t = time.Now()
	t.sendbuf.Push(pkt)

	if t.retrans == nil {
		// WARN: is this right?
		d := time.Duration(t.rtt + t.rttvar << 2)
		t.retrans = time.After(d * time.Microsecond)
	}
	return
}

func (t *Tunnel) resend (stopid int32, stop bool) (err error) {
	for _, p := range t.sendbuf {
		err = t.send_packet(p, false)
		if err != nil { return }
		if stop && (p.seq - stopid) >= 0 { return }
	}
	return
}

func (t *Tunnel) on_retrans () (err error) {
	t.retrans_count += 1
	if t.retrans_count > MAXRESEND {
		t.logger.Info("send packet more then maxretrans times")
		t.send(RST, []byte{})
		t.c_event <- EV_END
		return
	}

	err = t.resend(0, false)
	if err != nil { return }

	d := (t.rtt + t.rttvar << 2) * (1 << t.retrans_count)
	t.retrans = time.After(time.Duration(d) * time.Microsecond)
	return
}
