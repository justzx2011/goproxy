package tunnel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const HEADERSIZE = 13

var c_pktfree chan *Packet

func init () {
	c_pktfree = make(chan *Packet, 100)
}

func get_packet () (p *Packet) {
	select {
	case p = <- c_pktfree:
	default: p = new(Packet)
	}
	return
}

func put_packet (p *Packet) {
	select {
	case c_pktfree <- p:
	default:
	}
}

type Packet struct {
	flag uint8
	window uint16
	seq int32
	ack int32
	content []byte

	buf [SMSS+200]byte
	t time.Time
}

func half_packet(content []byte) (p *Packet) {
	p = get_packet()
	n := copy(p.buf[HEADERSIZE:], content)
	p.content = p.buf[HEADERSIZE:HEADERSIZE+n]
	return p
}

func (p *Packet) read_status (t *Tunnel) {
	t.readlck.Lock()
	l := t.readbuf.Len()
	t.readlck.Unlock()
	if WINDOWSIZE > l {
		p.window = uint16(WINDOWSIZE - l)
	}else{ p.window = 0 }
	p.seq = t.sendseq
	p.ack = t.recvseq
}

func (p *Packet) String() string {
	return fmt.Sprintf("flag: %s, seq: %d, ack: %d, wnd: %d, len: %d",
		DumpFlag(p.flag), p.seq, p.ack, p.window, len(p.content))
}

func (p *Packet) Pack() (n int, err error) {
	buf := bytes.NewBuffer(p.buf[:0])
	if len(p.content) > SMSS { return 0, errors.New("packet too large") }
	err = binary.Write(buf, binary.BigEndian, &p.flag)
	if err != nil { return }
	err = binary.Write(buf, binary.BigEndian, &p.window)
	if err != nil { return }
	err = binary.Write(buf, binary.BigEndian, &p.seq)
	if err != nil { return }
	err = binary.Write(buf, binary.BigEndian, &p.ack)
	if err != nil { return }
	err = binary.Write(buf, binary.BigEndian, uint16(len(p.content)))
	if err != nil { return }
	return HEADERSIZE+len(p.content), err
}

func (p *Packet) Unpack(n int) (err error) {
	var l uint16
	buf := bytes.NewBuffer(p.buf[:n])

	err = binary.Read(buf, binary.BigEndian, &p.flag)
	if err != nil { return }
	err = binary.Read(buf, binary.BigEndian, &p.window)
	if err != nil { return }
	err = binary.Read(buf, binary.BigEndian, &p.seq)
	if err != nil { return }
	err = binary.Read(buf, binary.BigEndian, &p.ack)
	if err != nil { return }
	err = binary.Read(buf, binary.BigEndian, &l)
	if err != nil { return }

	if buf.Len() < int(l) { return errors.New("packet broken") }
	if l > SMSS { return errors.New("packet too large") }
	p.content = buf.Bytes()
	return
}
