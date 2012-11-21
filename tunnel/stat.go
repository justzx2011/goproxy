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