package main

import (
	"fmt"
	"flag"
	"./dns"
)

func main() {
	flag.Parse()
	addrs, _ := dns.LookupIP(flag.Arg(0))
	fmt.Println(flag.Arg(0))
	for _, addr := range addrs {
		fmt.Printf("\t%s\n", addr)
	}
}