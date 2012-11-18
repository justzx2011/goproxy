package tunnel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
)

type Packet struct {
	flag uint8
	window uint16
	seq int32
	ack int32
	content []byte

	t time.Time
}

func NewPacket(t *Tunnel, flag uint8, content []byte) (p *Packet) {
	p = new(Packet)
	p.flag = flag
	if WINDOWSIZE > t.readlen {
		p.window = uint16(WINDOWSIZE - t.readlen)
	}else{ p.window = 0 }
	p.seq = t.sendseq
	p.ack = t.recvseq
	p.content = content
	return
}

func (p Packet) String() string {
	return fmt.Sprintf("flag: %s, seq: %d, ack: %d, wnd: %d, len: %d",
		DumpFlag(p.flag), p.seq, p.ack, p.window, len(p.content))
}

func (p Packet) Pack() (b []byte, err error) {
	var buf bytes.Buffer
	p.WriteTo(&buf)
	return buf.Bytes(), err
}

func (p *Packet) WriteTo(w io.Writer) (err error) {
	err = binary.Write(w, binary.BigEndian, &p.flag)
	if err != nil { return }
	err = binary.Write(w, binary.BigEndian, &p.window)
	if err != nil { return }
	err = binary.Write(w, binary.BigEndian, &p.seq)
	if err != nil { return }
	err = binary.Write(w, binary.BigEndian, &p.ack)
	if err != nil { return }
	err = binary.Write(w, binary.BigEndian, uint16(len(p.content)))
	if err != nil { return }
	_, err = w.Write(p.content)
	return
}

func Unpack(b []byte) (p *Packet, err error) {
	var n uint16
	p = new(Packet)
	buf := bytes.NewBuffer(b)

	err = binary.Read(buf, binary.BigEndian, &p.flag)
	if err != nil { return }
	err = binary.Read(buf, binary.BigEndian, &p.window)
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