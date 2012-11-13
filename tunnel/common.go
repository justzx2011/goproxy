package tunnel

import (
	"bytes"
	"net"
)

const (
	DEBUG = false
	PACKETSIZE = 512
	MAXRESEND = 5
	RETRANS_SACKCOUNT = 2
)

const (
	TM_MSL = 30000 // ms
	TM_FINWAIT = 10000 // ms
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

type PacketQueue []*Packet

func (ph *PacketQueue) Push(p *Packet) {
	*ph = append(*ph, p)
}

func (ph *PacketQueue) Pop() (p *Packet) {
	p = (*ph)[0]
	*ph = (*ph)[1:]
	return
}

type DataBlock struct {
	remote *net.UDPAddr
	buf []byte
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
