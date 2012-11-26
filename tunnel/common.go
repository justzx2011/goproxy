package tunnel

import (
	"fmt"
	"net"
	"strings"
	"strconv"
)

const (
	DEBUGMODE = true
	DROPFLAG = false
	DROPRATE = 90
	MSS = 1024 // Sender Maximum Segment Size
	TBUFSIZE = 32 // length of queue in read/write per tunnel
	READBUFSIZE = 1024
	WINDOWSIZE = READBUFSIZE * MSS
	RESTARTACK = 3*16*1024
	MAXRESEND = 13
	RETRANS_SACKCOUNT = 2
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

type PacketQueue []*Packet

func (pq *PacketQueue) Len() (int) {
	return len(*pq)
}

func (pq *PacketQueue) Push(pkt *Packet) (ok bool) {
	var i int

	for i = len(*pq)-1; i >= 0; i-- {
		df := (*pq)[i].seq - pkt.seq
		if df == 0 { return false }
		if df < 0 { break }
	}

	switch i {
	case len(*pq)-1: *pq = append(*pq, pkt)
	default:
		*pq = append(*pq, nil)
		copy((*pq)[i+2:], (*pq)[i+1:])
		(*pq)[i+1] = pkt
	}
	return true
}

func (pq *PacketQueue) Pop() (p *Packet) {
	p = (*pq)[0]
	*pq = (*pq)[1:]
	return
}

func (pq *PacketQueue) Front() (pkt *Packet) {
	return (*pq)[0]
}

func (pq *PacketQueue) Get (i int) (pkt *Packet) {
	if i >= len(*pq) {
		panic(fmt.Sprintf("index %d large then queue size %d",
			i, len(*pq)))
	}
	return (*pq)[i]
}

func (pq *PacketQueue) String () (s string) {
	var ss []string
	for _, p := range *pq {
		ss = append(ss, strconv.Itoa(int(p.seq)))
	}
	return "[" + strings.Join(ss, "|") + "]"
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