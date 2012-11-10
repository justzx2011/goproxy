package main

import (
	"errors"
	"fmt"
	"net"
)

func UdpServer(addr string, handler func (*net.UDPConn, *net.UDPAddr, []byte) (err error)) (err error) {
	var n int
	var buf []byte
	var remote *net.UDPAddr

	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil { return }

	for {
		buf = make([]byte, 2048)
		n, remote, err = conn.ReadFromUDP(buf)
		if err != nil { return }
		err = handler(conn, remote, buf[:n])
		if err != nil { return }
	}
	return
}

func main () {
	UdpServer(":1111", func (conn *net.UDPConn, remote *net.UDPAddr, data []byte) (err error){
		var n int
		fmt.Println(data)
		n, err = conn.WriteToUDP(data, remote)
		if n != len(data) {
			err = errors.New("size dismatch")
		}
		if err != nil { return }
		return
	})
}