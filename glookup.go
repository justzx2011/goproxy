package main

import (
	"fmt"
	"flag"
	"./dns"
)

func main() {
	var err error
	flag.Parse()
	err = dns.LoadConfig("resolv.conf")
	if err != nil { panic(err.Error()) }
	addrs, _ := dns.LookupIP(flag.Arg(0))
	fmt.Println(flag.Arg(0))
	for _, addr := range addrs {
		fmt.Printf("\t%s\n", addr)
	}
}