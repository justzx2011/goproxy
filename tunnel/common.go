package tunnel

import (
	"bytes"
	"net"
	"strings"
)

const (
	DROPFLAG = false
	PACKETSIZE = 512
	WINDOWSIZE = 16 * 1024
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
	SACK = uint8(0x80)
	RST = uint8(0x08)
	SYN = uint8(0x04)
	ACK = uint8(0x02)
	FIN = uint8(0x01)
)

const (
	EV_CONNECT = iota
	EV_CONNECTED
	EV_CLOSE
	EV_CLOSED
	EV_END
)

const (
	CLOSED = iota
	SYNRCVD
	SYNSENT
	EST
	FINWAIT1
	FINWAIT2
	CLOSING
	TIMEWAIT
	LASTACK
)

func DumpStatus(st uint8) string {
	switch st{
	case CLOSED: return "CLOSED"
	case SYNRCVD: return "SYNRCVD"
	case SYNSENT: return "SYNSENT"
	case EST: return "EST"
	case FINWAIT1: return "FINWAIT1"
	case FINWAIT2: return "FINWAIT2"
	case CLOSING: return "CLOSING"
	case TIMEWAIT: return "TIMEWAIT"
	case LASTACK: return "LASTACK"
	}
	return "UNKNOWN"
}

func DumpFlag(flag uint8) (r string) {
	var rs []string
	if (flag & SACK) != 0 { rs = append(rs, "SACK") }
	if (flag & RST) != 0 { rs = append(rs, "RST") }
	if (flag & SYN) != 0 { rs = append(rs, "SYN") }
	if (flag & ACK) != 0 { rs = append(rs, "ACK") }
	if (flag & FIN) != 0 { rs = append(rs, "FIN") }
	r = strings.Join(rs, "|")
	if r == "" { return "NON" }
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
