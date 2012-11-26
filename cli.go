package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	// "math/rand"
	"net"
	"os"
	"runtime"
	"time"
	"./sutils"
	"./tunnel"
)

const BLOCKSIZE = 1024

var data []byte
var serveraddr string

func init () {
	var logfile string
	var loglevel string

	flag.StringVar(&logfile, "logfile", "", "log file")
	flag.StringVar(&loglevel, "loglevel", "WARNING", "log level")
	flag.Parse()

	lv, err := sutils.GetLevelByName(loglevel)
	if err != nil { log.Fatal(err.Error()) }
	err = sutils.SetupLog(logfile, lv, 16)
	if err != nil { log.Fatal(err.Error()) }
}

func prepare_initdata () (err error) {
	data = make([]byte, BLOCKSIZE)

	f, err := os.Open("/dev/urandom")
	if err != nil { return }
	defer f.Close()

	n, err := f.Read(data)
	if err != nil { return }
	if n < BLOCKSIZE { return io.EOF }

	return
}

var changroup map[chan uint8]*tunnel.TunnelConn

func pre_client (c chan uint8) {
	var n int
	var err error
	var buf [BLOCKSIZE]byte
	var conn net.Conn

	conn, err = tunnel.DialTunnel(serveraddr)
	if err != nil {
		sutils.Err(err)
		return
	}
	changroup[c] = conn.(*tunnel.TunnelConn)
	defer func () {
		delete(changroup, c)
		conn.Close()
		sutils.Info("quit")
		go pre_client(c)
	}()
	sutils.Info("start")
	// max := rand.Intn(100)
	max := 100000

	for i := 0; i < max; i++ {
		_, err = conn.Write(data)
		if err == io.EOF { break }
		if err != nil {
			sutils.Err("Write", err)
			return
		}

		n, err = io.ReadFull(conn, buf[:])
		if err == io.EOF { break }
		if err != nil {
			sutils.Err("Read", err)
			return
		}

		if !bytes.Equal(buf[:n], data) {
			sutils.Err("response not match")
		}

		select {
		case <- c:
		default:
		}
		// time.Sleep(10 * time.Millisecond)
	}
}

func main () {
	runtime.GOMAXPROCS(12)
	var err error

	if len(flag.Args()) < 1 {
		log.Fatal("args not enough")
	}
	serveraddr = flag.Args()[0]

	err = prepare_initdata()
	if err != nil {
		sutils.Err("init", err)
		return
	}
	changroup = make(map[chan uint8]*tunnel.TunnelConn)

	for i := 0; i < 4; i++ {
		c := make(chan uint8, 2)
		go pre_client(c)
	}

	for {
		for c, tc := range changroup {
			if len(c) > 1 {
				sutils.Err("somebody die", tc)
			}
			select {
			case c <- 1:
			default:
			}
		}
		time.Sleep(60 * time.Second)
	}
}
