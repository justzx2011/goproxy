package tunnel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	PACKETSIZE = 512
	MAXRESEND = 3
)

type Packet struct {
	flag uint8
	seq int32
	ack int32
	content []byte
	timeout *time.Timer
	resend_count int
	t time.Time
}

const (
	SYN = uint8(0x04)
	ACK = uint8(0x02)
	FIN = uint8(0x01)
)

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

type PacketHeap []*Packet

func (ph PacketHeap) Len() int { return len(ph) }

func (ph PacketHeap) Less(i, j int) bool {
	return (ph[i].seq - ph[j].seq) < 0
}

func (ph PacketHeap) Swap(i, j int) {
	ph[i], ph[j] = ph[j], ph[i]
}

func (ph *PacketHeap) Push(x interface{}) {
	*ph = append(*ph, x.(*Packet))
}

func (ph *PacketHeap) Pop() interface{} {
	x := (*ph)[len(*ph)-1]
	*ph = (*ph)[:len(*ph)-1]
	return x
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
