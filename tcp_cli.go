package main

import (
	"flag"
	"io"
	"log"
	"net"
	"./sutils"
	"./secconn"
)

var key []byte
var iv []byte

var listenaddr string
var serveraddr string
var keyfile string

func init() {
	var err error

	flag.StringVar(&listenaddr, "listen", ":1080", "listen address")
	flag.StringVar(&serveraddr, "server", ":8899", "server address")
	flag.StringVar(&keyfile, "keyfile", "file.key", "key and iv file")
	flag.Parse()

	if len(flag.Args()) < 1 {
		log.Fatal("args not enough")
	}
	serveraddr = flag.Args()[0]

	key, iv, err = secconn.ReadKey(keyfile)
	if err != nil {
		log.Fatal(err.Error())
	}
}


func main() {
	sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		tcpAddr, err := net.ResolveTCPAddr("tcp4", serveraddr)
		if err != nil { return }
		dstconn, err := net.DialTCP("tcp4", nil, tcpAddr)
		if err != nil { return }

		secdst, err := secconn.NewConn(dstconn, key, iv)
		if err != nil { return }

		go func () {
			io.Copy(conn, secdst)
		}()
		io.Copy(secdst, conn)
		return
	})
}