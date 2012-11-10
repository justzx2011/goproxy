package tunnel

import (
	"bytes"
	"encoding/binary"
	"errors"
)

const PACKETSIZE = 512

type Packet struct {
	flag uint8
	seq int32
	content []byte
}

const (
	PKT_DATA = 0
	PKT_STATUS = 1
	PKT_GET = 2
	PKT_CLOSING = 4
	PKT_CLOSED = 5
)

func (p Packet)Pack() (b []byte, err error) {
	var buf bytes.Buffer
	err = binary.Write(&buf, binary.BigEndian, &p.flag)
	if err != nil { return }
	err = binary.Write(&buf, binary.BigEndian, &p.seq)
	if err != nil { return }
	err = binary.Write(&buf, binary.BigEndian, uint16(len(p.content)))
	if err != nil { return }

	var n int
	b = make([]byte, 7)
	n, err = buf.Read(b)
	if n != 7 { return nil, errors.New("header pack wrong") }
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
	err = binary.Read(buf, binary.BigEndian, &n)
	if err != nil { return }

	p.content = buf.Bytes()
	if uint16(len(p.content)) != n {
		return nil, errors.New("packet broken")
	}
	return
}
func SplitBytes(b []byte, size int, f func ([]byte) (error)) (err error) {
	var n int
	var bi []byte
	
	buf := bytes.NewBuffer(b)
	for buf.Len() > 0 {
		bi = make([]byte, size)
		n, err = buf.Read(bi)
		if err != nil { return }
		err = f(bi[:n])
		if err != nil { return }
	}
	return
}

func SplitBytes(b []byte, size int, f func ([]byte) (error)) (err error) {
	var n int
	var bi []byte
	
	buf := bytes.NewBuffer(b)
	for buf.Len() > 0 {
		bi = make([]byte, size)
		n, err = buf.Read(bi)
		if err != nil { return }
		err = f(bi[:n])
		if err != nil { return }
	}
	return
}
