package main

import (
	// "errors"
	"fmt"
	// "net"
	"./tunnel"
)

func main () {
	conn, err := tunnel.DialTunnel("127.0.0.1:1111")
	if err != nil {
		fmt.Println(err.Error())
	}
	conn.Write([]byte{0x01, 0x02})

	var n int
	var buf []byte
	buf = make([]byte, 100)
	n, err = conn.Read(buf)
	fmt.Println(buf[:n])
	conn.Close()
	return
}