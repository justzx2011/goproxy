package tunnel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

type Packet struct {
	flag uint8
	window uint32 // 24bits in fact
	seq int32
	ack int32
	content []byte

	t time.Time
}

func NewPacket(t *Tunnel, flag uint8, content []byte) (p *Packet) {
	p = new(Packet)
	p.flag = flag
	p.window = t.recvwnd
	p.seq = t.sendseq
	p.ack = t.recvseq
	p.content = content
	return
}

func (p Packet) Dump() string {
	return fmt.Sprintf("flag: %d, seq: %d, ack: %d, wnd: %d, len: %d",
		DumpFlag(p.flag), p.seq, p.ack, p.window, len(p.content))
}

func (p Packet) Pack() (b []byte, err error) {
	var h uint32
	var buf bytes.Buffer
	h = (uint32(p.flag) << 24) + (p.window & 0xffffff)
	err = binary.Write(&buf, binary.BigEndian, &h)
	if err != nil { return }
	err = binary.Write(&buf, binary.BigEndian, &p.seq)
	if err != nil { return }
	err = binary.Write(&buf, binary.BigEndian, &p.ack)
	if err != nil { return }
	err = binary.Write(&buf, binary.BigEndian, uint16(len(p.content)))
	if err != nil { return }
	_, err = buf.Write(p.content)
	return buf.Bytes(), err
}

func Unpack(b []byte) (p *Packet, err error) {
	var h uint32
	var n uint16
	p = new(Packet)
	buf := bytes.NewBuffer(b)

	err = binary.Read(buf, binary.BigEndian, &h)
	if err != nil { return }
	p.flag = uint8(h >> 24)
	p.window = h & 0xffffff
	err = binary.Read(buf, binary.BigEndian, &p.seq)
	if err != nil { return }
	err = binary.Read(buf, binary.BigEndian, &p.ack)
	if err != nil { return }
	err = binary.Read(buf, binary.BigEndian, &n)
	if err != nil { return }

	p.content = buf.Bytes()
	if uint16(len(p.content)) != n {
		return nil, errors.New("packet broken")
	}
	return
}