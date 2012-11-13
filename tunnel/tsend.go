package tunnel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

func NewTunnel(remote *net.UDPAddr) (t *Tunnel, err error) {
	t = new(Tunnel)
	t.remote = remote
	t.status = CLOSED

	t.c_recv = make(chan []byte, 10)

	t.sendseq = 0
	t.recvseq = 0
	t.recvack = 0
	t.sendbuf = make(PacketQueue, 0)
	t.recvbuf = make(PacketQueue, 0)

	t.rtt = 200000
	t.rttvar = 200000
	t.sack_count = 0
	t.retrans_count = 0
	t.keepalive = time.After(time.Duration(TM_KEEPALIVE) * time.Second)

	t.c_read = make(chan []byte, 1)
	t.c_write = make(chan []byte, 1)
	t.c_evin = make(chan uint8)
	t.c_evout = make(chan uint8)

	go t.main()
	return
}

func (t Tunnel) Dump() string {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprintf(buf,
		"status: %s, sendseq: %d, recvseq: %d, sendbuf: %d, recvbuf: %d, readbuf: %d, writebuf: %d",
		DumpStatus(t.status), t.sendseq, t.recvseq,
		len(t.sendbuf), len(t.recvbuf), len(t.c_read), len(t.c_write))
	return buf.String()
}

func (t *Tunnel) send_sack() (err error) {
	buf := bytes.NewBuffer([]byte{})
	for i, p := range t.recvbuf {
		if i > 0x7f { break }
		binary.Write(buf, binary.BigEndian, p.seq)
	}
	return t.send(SACK, buf.Bytes())
}

func (t *Tunnel) send(flag uint8, content []byte) (err error) {
	if t.recvack != t.recvseq { flag |= ACK }
	err = t.send_packet(NewPacket(t, flag, content), true)
	if err != nil { return }

	switch {
	case flag == SACK:
	case len(content) > 0: t.sendseq += int32(len(content))
	case flag != ACK: t.sendseq += 1
	}

	t.recvack = t.recvseq
	if t.delayack != nil { t.delayack = nil }
	if DEBUG { log.Println("send out", t.Dump()) }
	return
}

func (t *Tunnel) send_packet(pkt *Packet, retrans bool) (err error) {
	var buf []byte
	if DEBUG { log.Println("send in", pkt.Dump()) }

	buf, err = pkt.Pack()
	if err != nil { return }
	if DEBUG { log.Println("send", t.remote, buf) }

	t.c_send <- &DataBlock{t.remote, buf}
	if pkt.flag == SACK { return }
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
		log.Println("send packet more then maxretrans times")
		t.Close()
		return
	}

	err = t.resend(0, false)
	if err != nil { return }

	d := (t.rtt + t.rttvar << 2) * (1 << t.retrans_count)
	t.retrans = time.After(time.Duration(d) * time.Microsecond)
	return
}

func (t *Tunnel) Close () (err error) {
	if t.onclose != nil { t.onclose() }
	t.c_evin <- END
	close(t.c_send)
	return
}