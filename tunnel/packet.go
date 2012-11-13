package tunnel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	DEBUG = false
	PACKETSIZE = 512
	MAXRESEND = 5
	RETRANS_SACKCOUNT = 2
)

const (
	TM_MSL = 10000 // ms
	TM_FINWAIT = 2000 // ms
	TM_KEEPALIVE = 3600 // s
	TM_DELAYACK = 200 // ms
	TM_CONNEST = 75 // s
)

const (
	SACK = uint8(0x10)
	SYN = uint8(0x04)
	ACK = uint8(0x02)
	FIN = uint8(0x01)
	END = uint8(0xff)
)

const (
	CLOSED = 0
	SYNRCVD = 1
	SYNSENT = 2
	EST = 3
	FINWAIT = 4
	TIMEWAIT = 5
	LASTACK = 6
)

func DumpStatus(st uint8) string {
	switch st{
	case CLOSED: return "CLOSED"
	case SYNRCVD: return "SYNRCVD"
	case SYNSENT: return "SYNSENT"
	case EST: return "EST"
	case FINWAIT: return "FINWAIT"
	case TIMEWAIT: return "TIMEWAIT"
	case LASTACK: return "LASTACK"
	}
	return "unknown"
}

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

type PacketQueue []*Packet

func (ph *PacketQueue) Push(p *Packet) {
	*ph = append(*ph, p)
}

func (ph *PacketQueue) Pop() (p *Packet) {
	p = (*ph)[0]
	*ph = (*ph)[1:]
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
