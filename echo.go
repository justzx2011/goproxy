package main

import (
	"flag"
	"io"
	"log"
	"math/rand"
	"net"
	"time"
	"./sutils"
	"./tunnel"
)

var listenaddr string

func init () {
	var logfile string
	var loglevel string

	flag.StringVar(&logfile, "logfile", "", "log file")
	flag.StringVar(&loglevel, "loglevel", "WARNING", "log level")
	flag.StringVar(&listenaddr, "listen", ":8899", "listen address")
	flag.Parse()

	lv, err := sutils.GetLevelByName(loglevel)
	if err != nil { log.Fatal(err.Error()) }
	err = sutils.SetupLog(logfile, lv, 16)
	if err != nil { log.Fatal(err.Error()) }
}

var changroup map[chan uint8]uint8

func keepalive () {
	for {
		for c, _ := range changroup {
			if len(c) > 1 {
				sutils.Err("somebody die")
			}
			select {
			case c <- 1:
			default:
			}
		}
		time.Sleep(60 * time.Second)
	}
}

func main () {
	changroup = make(map[chan uint8]uint8)
	go keepalive()

	err := tunnel.UdpServer(listenaddr, func (conn net.Conn) (err error) {
		var n int
		var buf [2048]byte
		sutils.Info("connection comein")
		c := make(chan uint8, 2)
		changroup[c] = 0
		defer func () {
			conn.Close()
			sutils.Info("connnction breaking")
			delete(changroup, c)
		}()
		max := rand.Intn(100)

		for i := 0; i < max; i++ {
			n, err = conn.Read(buf[:])
			if err == io.EOF {
				return
			}
			if err != nil {
				sutils.Err(err)
				return
			}

			_, err = conn.Write(buf[:n])
			if err != nil {
				sutils.Err(err)
				return
			}

			select {
			case <- c:
			default:
			}
		}
		return
	})
	if err != nil {
		sutils.Err(err)
		return
	}
}