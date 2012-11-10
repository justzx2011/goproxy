package main

import (
	"errors"
	"fmt"
	"net"
)

func test01 (addr string) (err error) {
	var conn *net.UDPConn
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	conn, err = net.DialUDP("udp", nil, udpaddr)
	if err != nil { return }

	data := []byte{0x01, 0x02, 0x03}
	n, err := conn.Write(data)
	if n != len(data) {
		err = errors.New("size dismatch")
	}
	if err != nil { return }

	data = make([]byte, 2048)
	n, err = conn.Read(data)
	fmt.Println(data[:n])
	return 
}

func main () {
	err := test01("127.0.0.1:1111")
	if err != nil {
		fmt.Println(err.Error())
	}
}