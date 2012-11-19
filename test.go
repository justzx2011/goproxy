package main

import (
	// "bytes"
	"fmt"
	// "log"
	// "net"
	// "./sutils"
	// "./tunnel"
)

func main () {
	var pkt *Packet
	var pq PacketQueue

	pkt = new(Packet)
	pkt.seq = 0
	pq.Push(pkt)

	pkt = new(Packet)
	pkt.seq = 3
	pq.Push(pkt)

	for _, pkt = range pq {
		fmt.Println(pkt.seq)
	}

	fmt.Println()

	pkt = new(Packet)
	pkt.seq = 1
	pq.Push(pkt)
	
	for _, pkt = range pq {
		fmt.Println(pkt.seq)
	}
}