package tunnel

import (
	"fmt"
)

type Statistics struct {
	sendpkt uint64
	sendsize uint64
	senderr uint64
	recvpkt uint64
	recvsize uint64
	recverr uint64
}

func (s *Statistics) String () (string) {
	return fmt.Sprintf("spkt: %d, ssz: %d, serr: %d, rpkt: %d, rsz: %d, rerr: %d",
		s.sendpkt, s.sendsize, s.senderr, s.recvpkt, s.recvsize, s.recverr)
}

type TcpStatistics struct {
	rexmt uint64
	rexmtsize uint64

	sack uint64
	sacksize uint64

	rttupdated uint64
	sendack uint64
	
}