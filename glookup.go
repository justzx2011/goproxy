package main

import (
	"fmt"
	"flag"
	"net"
	"./dns"
	// "./sutils"
)

var blackfile string
var listenaddr string

var cryptWrapper func (net.Conn) (net.Conn, error) = nil

func loaddns() {
}

func main() {
	var err error
	flag.Parse()
	err = dns.LoadConfig("resolv.conf")
	if err != nil { panic(err.Error()) }
	blackfile = "routes.list.gz"
	readlist()

	addrs, _ := dns.LookupIP(flag.Arg(0))
	fmt.Println(flag.Arg(0))
	for _, addr := range addrs {
		fmt.Printf("\t%s\t%t\n", addr, list_contain(blacklist, addr))
	}
}