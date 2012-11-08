package main

import (
	"flag"
	"log"
	"net"
	"./socks"
	"./sutils"
	"./secconn"
)

var key []byte
var iv []byte

var socksaddr string
var passfile string
var keyfile string

func init() {
	var err error
	flag.StringVar(&socksaddr, "socks", ":8899", "socksv5 address")
	flag.StringVar(&passfile, "passfile", "", "password file")
	flag.StringVar(&keyfile, "keyfile", "file.key", "key and iv file")
	flag.Parse()

	key, iv, err = secconn.ReadKey(keyfile)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func main() {
	ap := socks.NewAuthPassword()
	if len(passfile) > 0 { ap.LoadFile(passfile) }
	sutils.TcpServer(socksaddr, func (conn net.Conn) (err error) {
		secsrc, err := secconn.NewConn(conn, key, iv)
		if err != nil { return }
		return ap.Handler(secsrc)
		return
	})
	return
}