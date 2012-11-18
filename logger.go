package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"strconv"
	"time"
	"./sutils"
)

// const blocksize = 65536
const BLOCKSIZE = 512

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

type FlushBlock struct {
	key string
	buf bytes.Buffer
}

func log_writer (c chan *FlushBlock) {
	var fb * FlushBlock
	for {
		fb = <- c
		f, err := os.OpenFile(fb.key+".log", os.O_RDWR | os.O_APPEND | os.O_CREATE, 0644)
		if err != nil {
			log.Println(err.Error())
		}
		fb.buf.WriteTo(f)
		f.Close()
	}
	return
}

var re_data *regexp.Regexp
func log_analyzer (b []byte) (key string, l string, err error) {
	ss := re_data.FindStringSubmatch(string(b))

	h, err := strconv.Atoi(ss[1])
	if err != nil { return }

	if (h % 8) > loglv { return }
	pri, ok := lvname[h%8]
	if !ok {
		err = errors.New("lvname not found")
		return
	}

	header := strings.Split(ss[2], " ")
	timestamp := header[0]
	hostname := header[1]
	procid := header[2]
	msgid := header[3]
	msg := ss[3]

	key = hostname + "_" + procid
	l = fmt.Sprintf("%s %s[%s]: %s\n", timestamp, msgid, pri, msg)
	// key = hostname + "_" + msgid
	// l = fmt.Sprintf("%s [%s]: %s\n", timestamp, pri, msg)
	return
}

func main () {
	var err error

	c := make(chan []byte, 1000000)
	go udpserver(listenaddr, c)
	cw := make(chan *FlushBlock, 1000000)
	go log_writer(cw)

	var b []byte
	re_data, err = regexp.Compile("\\<(\\d+)\\>(.*?)\\[\\]: (.*)\n")
	if err != nil {
		log.Fatal("regex failed")
	}

	var ok bool
	var fb *FlushBlock
	var time_flush  <-chan time.Time
	fmap := make(map[string]*FlushBlock)
	counter := 0
	recv_counter := 0

	for {
		select {
		case b = <- c:
			recv_counter += 1
			key, l, err := log_analyzer(b)
			if err != nil {
				log.Println(err.Error())
				continue
			}
			if len(key) == 0 { continue }

			fb, ok = fmap[key]
			if !ok {
				fb = new(FlushBlock)
				fb.key = key
				fmap[key] = fb
			}

			fb.buf.WriteString(l)

			if fb.buf.Len() > BLOCKSIZE {
				delete(fmap, key)
				cw <- fb
			}

			counter += 1
			// if (recv_counter % 10) == 0 { fmt.Printf("%d/%d.\r", counter, recv_counter) }
			time_flush = time.After(time.Duration(3) * time.Second)
		case <- time_flush:
			fmt.Println("flush out\n")
			for _, v := range fmap { cw <- v }
			fmap = make(map[string]*FlushBlock)
		}
	}		
}
