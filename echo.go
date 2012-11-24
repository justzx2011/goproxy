package main

import (
	"flag"
	"net"
	"./sutils"
	"./tunnel"
)

var listenaddr string

func init () {
	var logfile string

	flag.StringVar(&listenaddr, "listen", ":8899", "listen address")
	flag.StringVar(&logfile, "logfile", "", "log file")
	flag.Parse()

	sutils.SetupLog(logfile, sutils.LOG_DEBUG, 16)
	return
}

func main () {
	err := tunnel.UdpServer(listenaddr, func (conn net.Conn) (err error) {
		defer conn.Close()

		var n int
		var buf [2048]byte

		n, err = conn.Read(buf[:])
		if err != nil {
			sutils.Err(err)
			return
		}

		_, err = conn.Write(buf[:n])
		if err != nil {
			sutils.Err(err)
			return
		}
		return
	})
	if err != nil {
		sutils.Err(err)
		return
	}
}