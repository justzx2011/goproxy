package tunnel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"time"
)

var send_count int64
var sendpkt_count int64

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
	send_count += 1

	// todo: status djuge

	if t.status != EST && flag == 0{
		return fmt.Errorf("can't send data, %s, pkt: %s", t, DumpFlag(flag))
	}

	if pkt == nil {
		pkt = get_packet()
		pkt.content = pkt.buf[HEADERSIZE:HEADERSIZE]
	}
	if t.recvack != t.recvseq { flag |= ACK }
	pkt.read_status(t, flag)
	size := len(pkt.content)

	t.send_packet(pkt)
	t.recvack = t.recvseq
	if t.t_dack != 0 { t.t_dack = 0 }

	// todo: 不加入sendbuf的pkt的回收
	// 不能直接回收，会导致发送时有问题
	switch { // 不参与seq递增的包，也不需要retrans
	case (flag & SACK) != 0: return
	case size > 0: t.sendseq += int32(size)
	case flag != ACK: t.sendseq += 1
	default: return
	}

	pkt.t = time.Now()
	t.sendbuf.Push(pkt)
	if t.t_rexmt == 0 { t.t_rexmt = int32(t.rtt + t.rttvar << 2) }
	return
}

func (t *Tunnel) send_packet(pkt *Packet) {
	sendpkt_count += 1
	t.logger.Debug("send", pkt)
	t.logger.Debug("send rate,", send_count, sendpkt_count)

	if DROPFLAG && rand.Intn(100) >= DROPRATE {
		t.logger.Debug("drop packet")
	}else{
		t.c_send <- &SendBlock{t.remote, pkt}
	}
	return
}

func (t *Tunnel) resend (stopid int32, stop bool) (err error) {
	for _, p := range t.sendbuf {
		t.send_packet(p)
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
	t.ssthresh = max32(inairlen >> 2, 2*SMSS)
	t.logger.Debug("congestion adjust, resend,", t.cwnd, t.ssthresh)

	t.t_rexmt = int32(t.rtt + t.rttvar << 2) * (1 << t.retrans_count)
	return
}
