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
	seq int32
	ack int32
	content []byte

	t time.Time
	timeout *time.Timer
	resend_count int
}

func NewPacket(t *Tunnel, flag uint8, content []byte) (p *Packet) {
	p = new(Packet)
	p.flag = flag
	p.seq = t.sendseq
	p.ack = t.recvseq
	p.content = content
	return
}

func (p Packet) Dump() string {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprintf(buf, "flag: %d, seq: %d, ack: %d, len:%d",
		p.flag, p.seq, p.ack, len(p.content))
	return buf.String()
}

func (p Packet) Pack() (b []byte, err error) {
	var buf bytes.Buffer
	err = binary.Write(&buf, binary.BigEndian, &p.flag)
	if err != nil { return }
	err = binary.Write(&buf, binary.BigEndian, &p.seq)
	if err != nil { return }
	err = binary.Write(&buf, binary.BigEndian, &p.ack)
	if err != nil { return }
	err = binary.Write(&buf, binary.BigEndian, uint16(len(p.content)))
	if err != nil { return }

	var n int
	b = make([]byte, 11)
	n, err = buf.Read(b)
	if n != 11 { return nil, errors.New("header pack wrong") }
	b = append(b, p.content...)
	return
}

func Unpack(b []byte) (p *Packet, err error) {
	var n uint16
	p = new(Packet)
	buf := bytes.NewBuffer(b)

	err = binary.Read(buf, binary.BigEndian, &p.flag)
	if err != nil { return }
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