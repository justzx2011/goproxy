package tunnel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"time"
)

func (t *Tunnel) send_sack () (err error) {
	// t.logger.Warning("sack send", t.recvbuf.String())
	t.logger.Debug("sack send")
	pkt := get_packet()
	buf := bytes.NewBuffer(pkt.buf[HEADERSIZE:HEADERSIZE])
	for i, p := range t.recvbuf {
		if i >= SMSS/4 - 1 { break }
		binary.Write(buf, binary.BigEndian, p.seq)
	}
	pkt.content = buf.Bytes()
	return t.send(SACK, pkt)
}

func (t *Tunnel) send (flag uint8, pkt *Packet) (err error) {
	if t.status != EST && flag == 0{
		return fmt.Errorf("can't send data, %s, pkt: %s", t, DumpFlag(flag))
	}

	if pkt == nil {
		pkt = get_packet()
		pkt.content = pkt.buf[HEADERSIZE:HEADERSIZE]
	}
	if t.recvack != t.recvseq { flag |= ACK }
	pkt.flag = flag
	pkt.read_status(t)
	size := len(pkt.content)

	retrans := (flag & SACK) == 0 && (flag != ACK || size != 0)
	err = t.send_packet(pkt, retrans)
	if err != nil { return }
	// todo: 不加入sendbuf的pkt的回收
	// 不能直接回收，会导致发送时有问题
	// if !retrans { put_packet(pkt) }

	switch {
	case (flag & SACK) != 0:
	case size > 0: t.sendseq += int32(size)
	case flag != ACK: t.sendseq += 1
	}
	t.check_windows_block()

	t.recvack = t.recvseq
	if t.t_dack != 0 { t.t_dack = 0 }
	return
}

func (t *Tunnel) send_packet(pkt *Packet, retrans bool) (err error) {
	t.logger.Debug("send", pkt)

	if DROPFLAG && rand.Intn(100) >= 85 {
		t.logger.Debug("drop packet")
	}else{
		t.c_send <- &SendBlock{t.remote, pkt}
	}
	if !retrans { return }

	pkt.t = time.Now()
	t.sendbuf.Push(pkt)

	if t.t_rexmt == 0 {
		t.t_rexmt = int32(t.rtt + t.rttvar << 2)
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
		t.logger.Warning("send packet more then maxretrans times")
		t.send(RST, nil)
		t.c_event <- EV_END
		return
	}

	err = t.resend(0, false)
	if err != nil { return }

	inairlen := int32(0)
	if len(t.sendbuf) > 0 { inairlen = t.sendseq - t.sendbuf[0].seq }
	t.ssthresh = max32(inairlen/2, 2*SMSS)

	t.t_rexmt = int32(t.rtt + t.rttvar << 2) * (1 << t.retrans_count)
	return
}

func (t *Tunnel) check_windows_block () {
	inairlen := int32(0)
	if len(t.sendbuf) > 0 { inairlen = t.sendseq - t.sendbuf[0].seq }
	switch {
	case (inairlen >= t.sendwnd) || (inairlen >= t.cwnd):
		t.logger.Debug("blocking,", inairlen, t.sendwnd, t.cwnd, t.ssthresh)
		t.c_write = nil
	case t.status == EST && t.c_write == nil:
		t.logger.Debug("restart,", inairlen, t.sendwnd, t.cwnd, t.ssthresh)
		t.c_write = t.c_wrbak
	}
}