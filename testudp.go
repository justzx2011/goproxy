package main

import (
	// "errors"
	"fmt"
	"log"
	"net"
	"./tunnel"
)

func main () {
	err := tunnel.UdpServer(":1111", func (conn net.Conn) (err error){
		var n, ns int
		var buf []byte

		buf = make([]byte, 1000)
		n, err = conn.Read(buf)
		if err != nil { return }

		log.Println("response", buf[:n])

		ns, err = conn.Write(buf[:n])
		if n != ns {
			log.Println("send wrong")
		}
		if err != nil { return }
		return
	})
	fmt.Println(err.Error())
}