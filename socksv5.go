package main

import (
	"flag"
	"net"
	"./socks"
	"./sutils"
)

var socksaddr string
var passfile string

func init() {
	flag.StringVar(&socksaddr, "socks", ":1080", "socksv5 address")
	flag.StringVar(&passfile, "passfile", "", "password file")
	flag.Parse()
}

func main() {
	ap := socks.NewAuthPassword()
	if len(passfile) > 0 { ap.LoadFile(passfile) }
	sutils.TcpServer(socksaddr, func (conn net.Conn) (err error){
		return ap.Handler(conn)
	})
}