package main

import (
	"fmt"
	"flag"
	"net"
	"./dns"
	// "./sutils"
)

var listenaddr string
var username string
var password string
var passfile string
var blackfile string

var cryptWrapper func (net.Conn) (net.Conn, error) = nil

func main() {
	blackfile = "routes.list.gz"
	init_dail()

	flag.Parse()
	addrs, _ := dns.LookupIP(flag.Arg(0))
	fmt.Println(flag.Arg(0))
	for _, addr := range addrs {
		fmt.Printf("\t%s\t%t\n", addr, blacklist.Contain(addr))
	}
}