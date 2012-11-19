package tunnel

import (
	"net"
	"strings"
	"strconv"
)

const (
	DROPFLAG = false
	MAXPARELLELCONN = 800
	SMSS = 1024 // Sender Maximum Segment Size
	WINDOWSIZE = 65535
	READBUFSIZE = 100
	RESTARTACK = 3*16*1024
	MAXRESEND = 5
	RETRANS_SACKCOUNT = 2
)

const (
	TM_CLIMSL = 2000 // ms
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
	EV_READ
	EV_CLEANUP
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

func (pq *PacketQueue) Push(pkt *Packet) {
	var i int

	for i = len(*pq)-1; i >= 0; i-- {
		if ((*pq)[i].seq - pkt.seq) < 0 { break }
	}

	switch i {
	case len(*pq)-1: *pq = append(*pq, pkt)
	default:
		*pq = append(*pq, nil)
		copy((*pq)[i+2:], (*pq)[i+1:])
		(*pq)[i+1] = pkt
	}
}

func (pq *PacketQueue) Pop() (p *Packet) {
	p = (*pq)[0]
	*pq = (*pq)[1:]
	return
}

func (pq *PacketQueue) String () (s string) {
	for _, p := range *pq { s += strconv.Itoa(int(p.seq)) + "|" }
	return
}

type SendBlock struct {
	remote *net.UDPAddr
	pkt *Packet
}

func min(a, b int) int {
	if a < b { return a }
	return b
}

func max(a, b int) int {
	if a > b { return a }
	return b
}

func max32(a, b int32) int32 {
	if a > b { return a }
	return b
}

func abs(a int32) int32 {
	if a < 0 { return -a }
	return a
}