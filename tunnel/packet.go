package tunnel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"time"
)

const HEADERSIZE = 17

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
	crc uint32
	content []byte

	buf [SMSS+200]byte
	t time.Time
}

func half_packet(content []byte) (n int, p *Packet) {
	p = get_packet()
	n = copy(p.buf[HEADERSIZE:], content)
	p.content = p.buf[HEADERSIZE:HEADERSIZE+n]
	return
}

func (p *Packet) read_status (t *Tunnel, flag uint8) {
	p.flag = flag
	t.readlck.Lock()
	l := t.readbuf.Len()
	t.readlck.Unlock()
	if WINDOWSIZE > l {
		p.window = uint16(WINDOWSIZE - l)
	}else{ p.window = 0 }
	p.seq = t.sendseq
	p.ack = t.recvseq
	p.crc = crc32.ChecksumIEEE(p.content)
}

func (p *Packet) String() string {
	return fmt.Sprintf("flag: %s, seq: %d, ack: %d, wnd: %d, len: %d",
		DumpFlag(p.flag), p.seq, p.ack, p.window, len(p.content))
}

func (p *Packet) Pack() (n int, err error) {
	buf := bytes.NewBuffer(p.buf[:0])
	if len(p.content) > SMSS {
		fmt.Println(p)
		return 0, fmt.Errorf("packet too large, %d/%d", len(p.content), SMSS)
	}
	err = binary.Write(buf, binary.BigEndian, &p.flag)
	if err != nil { return }
	err = binary.Write(buf, binary.BigEndian, &p.window)
	if err != nil { return }
	err = binary.Write(buf, binary.BigEndian, &p.seq)
	if err != nil { return }
	err = binary.Write(buf, binary.BigEndian, &p.ack)
	if err != nil { return }
	err = binary.Write(buf, binary.BigEndian, &p.crc)
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
	err = binary.Read(buf, binary.BigEndian, &p.crc)
	if err != nil { return }
	err = binary.Read(buf, binary.BigEndian, &l)
	if err != nil { return }

	if l > SMSS { return fmt.Errorf("packet too large, %d/%d", l, SMSS) }
	if buf.Len() != int(l) { return errors.New("packet broken") }
	p.content = buf.Bytes()

	if p.crc != crc32.ChecksumIEEE(p.content) {
		return fmt.Errorf("crc32 fault %x/%x %s",
			p.crc, crc32.ChecksumIEEE(p.content), p.String())
	}
	return
}
