package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"strconv"
	"./sutils"
)

var lvname = map[int]string {
	0: "EMERG", 1: "ALERT", 2: "CRIT",
	3: "ERROR", 4: "WARNING", 5: "NOTICE",
	6: "INFO", 7: "DEBUG",
}

var listenaddr string
var loglv int

func init() {
	var err error
	var loglevel string

	flag.StringVar(&listenaddr, "listen", ":4455", "syslog listen addr")
	flag.StringVar(&loglevel, "loglevel", "DEBUG", "log level")
	flag.Parse()

	loglv, err = sutils.GetLevelByName(loglevel)
	if err != nil { log.Fatal(err.Error()) }
}

var recv_counter int

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
		recv_counter += 1
	}
}

func main () {
	var err error

	c := make(chan []byte, 1000000)
	go udpserver(listenaddr, c)

	var buf []byte
	re_data, err := regexp.Compile("\\<(\\d+)\\>(.*?)\\[\\]: (.*)\n")
	if err != nil {
		log.Fatal("regex failed")
	}

	var ok bool
	var pri string
	var fo *os.File
	counter := 0
	fmap := make(map[string]*os.File)

	for {
		buf = <- c
		ss := re_data.FindStringSubmatch(string(buf))

		h, err := strconv.Atoi(ss[1])
		if err != nil { continue }

		if (h % 8) > loglv { continue }
		pri, ok = lvname[h%8]
		if !ok { continue }

		header := strings.Split(ss[2], " ")
		timestamp := header[0]
		hostname := header[1]
		// procid := header[2]
		msgid := header[3]
		key := hostname + "_" + msgid

		fo, ok = fmap[key]
		if !ok {
			fo, err = os.Create(key+".log")
			if err != nil {
				log.Println("open file failed,", key+".log")
				continue
			}
			fmap[key] = fo
		}

		fo.WriteString(fmt.Sprintf("%s [%s]: %s\n", timestamp, pri, ss[3]))

		counter += 1
		fmt.Printf("processed %d/%d...\r", counter, recv_counter)
	}
}