package tunnel

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"time"
)

func (t *Tunnel) send_sack () (err error) {
	t.logger.Debug("sack send", t.recvbuf)
	pkt := get_packet()
	buf := bytes.NewBuffer(pkt.buf[HEADERSIZE:HEADERSIZE])
	for i, p := range t.recvbuf {
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

	t.send_packet(pkt)

	// 不参与seq递增的包，也不需要retrans
	switch flag & ACKMASK {
	case SACK: return
	case DAT:
		if size == 0 { return }
		t.sendseq += int32(size)
	default: t.sendseq += 1
	}
	// TODO: 不加入sendbuf的pkt的回收
	// 不能直接回收，会导致发送时有问题

	if !t.sendbuf.Push(pkt) { panic(pkt) }
	if t.timer.rexmt == 0 { t.timer.rexmt = t.rto }
	return
}

func (t *Tunnel) send_packet(pkt *Packet) {
	t.stat.senderr += 1
	pkt.sndtime = int32(time.Now().UnixNano()/NETTICK)
	pkt.acktime = t.recent
	t.logger.Debug("send", pkt)

	if DROPFLAG && rand.Intn(100) >= DROPRATE {
		t.logger.Debug("drop packet")
	}else{
		t.c_send <- &SendBlock{t.remote, pkt}
	}
	return
}

func (t *Tunnel) on_retrans () {
	t.retrans_count += 1
	if t.retrans_count > MAXRESEND {
		t.drop("send packet more then maxretrans times")
		return
	}

	// todo: put them into a pipe
	for _, p := range t.sendbuf { t.send_packet(p) }
	t.sack_count = 0
	t.sack_sent = nil

	inairlen := int32(0)
	if len(t.sendbuf) > 0 { inairlen = t.sendseq - t.sendbuf.Front().seq }
	t.ssthresh = max32(inairlen/2, 2*MSS)
	t.logger.Info("congestion adjust, resend,", t.cwnd, t.ssthresh)

	t.timer.rexmt = t.rto * (1 << t.retrans_count)
	return
}
