package sutils

import (
	"log"
	"net"
)

func TcpServer(addr string, handler func (conn net.Conn) (err error)) (err error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", addr)
	if err != nil { return }
	listener, err := net.ListenTCP("tcp4", tcpAddr)
	if err != nil { return }
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
			continue
		}
		go func () {
			e := handler(conn)
			if e != nil { log.Println(e.Error()) }
		} ()
	}
	return
}