package main

import (
	"fmt"
	"net"
)

func main () {
	udpaddr, err := net.ResolveUDPAddr("udp", "localhost:8899")
	if err != nil { return }
	conn, err := net.DialUDP("udp", nil, udpaddr)
	if err != nil { return }
	localaddr := conn.LocalAddr()
	fmt.Println(localaddr.String())
}