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
	if t.status != EST && flag == 0 { panic(flag) }

	if pkt == nil {
		pkt = get_packet()
		pkt.content = pkt.buf[HEADERSIZE:HEADERSIZE]
	}
	if t.recvack != t.recvseq { flag |= ACK }
	pkt.read_status(t, flag)
	size := len(pkt.content)

	t.send_packet(pkt)
	t.recvack = t.recvseq
	if t.timer.dack != 0 { t.timer.dack = 0 }

	// TODO: 不加入sendbuf的pkt的回收
	// 不能直接回收，会导致发送时有问题
	switch { // 不参与seq递增的包，也不需要retrans
	case (flag & SACK) != 0: return
	case size > 0: t.sendseq += int32(size)
	case flag != ACK: t.sendseq += 1
	default: return
	}

	pkt.t = time.Now()
	if !t.sendbuf.Push(pkt) { panic(pkt) }

	if t.timer.rexmt == 0 { t.timer.rexmt = t.rto }
	return
}

func (t *Tunnel) send_packet(pkt *Packet) {
	t.stat.senderr += 1
	t.logger.Debug("send", pkt)

	if DROPFLAG && rand.Intn(100) >= DROPRATE {
		t.logger.Debug("drop packet")
	}else{
		t.c_send <- &SendBlock{t.remote, pkt}
	}
	return
}

// FIXME: 高速网络中，rtt的速度太快，导致retrans调用来不及跟踪
func (t *Tunnel) on_retrans () (err error) {
	t.retrans_count += 1
	if t.retrans_count > MAXRESEND {
		t.drop()
		t.logger.Warning("send packet more then maxretrans times")
		return
	}

	for _, p := range t.sendbuf { t.send_packet(p) }

	inairlen := int32(0)
	if len(t.sendbuf) > 0 { inairlen = t.sendseq - t.sendbuf[0].seq }
	t.ssthresh = max32(int32(float32(inairlen)*BACKRATE), 2*MSS)
	t.logger.Info("congestion adjust, resend,", t.cwnd, t.ssthresh)

	t.timer.rexmt = t.rto * (1 << t.retrans_count)
	return
}
