package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
)

var lvname = map[int]string {
	0: "EMERG", 1: "ALERT", 2: "CRIT",
	3: "ERROR", 4: "WARNING", 5: "NOTICE",
	6: "INFO", 7: "DEBUG",
}

var listenaddr string

func init() {
	flag.StringVar(&listenaddr, "listen", ":4455", "syslog listen addr")
	flag.Parse()
}

func udpserver (addr string, c chan []byte) {
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil { return }
	defer conn.Close()

	var n int
	var buf []byte

	for {
		buf = make([]byte, 2048)
		n, _, err = conn.ReadFromUDP(buf)

		if err != nil { return }
		c <- buf[:n]
	}
}

func main () {
	var err error

	// if len(flag.Args()) < 1 {
	// 	log.Fatal("args not enough")
	// }		

	c := make(chan []byte, 10000)
	go udpserver(listenaddr, c)

	var buf []byte
	re_data, err := regexp.Compile("\\<(\\d+)\\>(.*) \\[(.*)\\] (.*)\n")
	if err != nil {
		log.Fatal("regex failed")
	}

	var ok bool
	var fn string
	var fo *os.File
	fmap := make(map[string]*os.File)

	for {
		buf = <- c
		ss := re_data.FindStringSubmatch(string(buf))

		f, err := strconv.Atoi(ss[1])
		if err != nil { continue }
		fn, ok = lvname[f]
		if !ok { continue }

		fo, ok = fmap[ss[3]]
		if !ok {
			fo, err = os.Create(ss[3]+".log")
			if err != nil {
				log.Println("open file failed,", ss[3]+".log")
				continue
			}
			fmap[ss[3]] = fo
		}

		fo.WriteString(fmt.Sprintf("%s [%s] %s\n", ss[2], fn, ss[4]))
	}
}