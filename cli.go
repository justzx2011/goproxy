package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"./sutils"
	"./tunnel"
)

var data []byte

func init () {
	var logfile string

	flag.StringVar(&logfile, "logfile", "", "log file")
	flag.Parse()

	sutils.SetupLog(logfile, sutils.LOG_DEBUG, 16)
}

func prepare_initdata () (err error) {
	data = make([]byte, 16384)

	f, err := os.Open("/dev/urandom")
	if err != nil { return }
	defer f.Close()

	n, err := f.Read(data)
	if err != nil { return }
	if n < 16384 { return io.EOF }

	return
}

func pre_client (conn net.Conn, wg sync.WaitGroup) {
	var n int
	var err error
	var buf [16384]byte
	defer conn.Close()

	for {
		_, err = conn.Write(data)
		if err != nil {
			sutils.Err(err)
			return
		}

		n, err = conn.Write(buf[:])
		if err != nil {
			sutils.Err(err)
			return
		}

		if !bytes.Equal(buf[:n], data) {
			sutils.Err("response not match")
		}
	}

	wg.Done()
}

func main () {
	var err error
	var serveraddr string
	var conn net.Conn

	if len(flag.Args()) < 1 {
		log.Fatal("args not enough")
	}
	serveraddr = flag.Args()[0]

	err = prepare_initdata()
	if err != nil {
		sutils.Err(err)
		return
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		conn, err = tunnel.DialTunnel(serveraddr)
		if err != nil { sutils.Err(err) }
		go pre_client(conn, wg)
	}

	wg.Wait()
}
