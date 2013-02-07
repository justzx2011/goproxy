package main

import (
	"flag"
	// "fmt"
	"io"
	"log"
	"net"
	// "os"
	"./socks"
	"./sutils"
	"./cryptconn"
)

var cipher string
var keyfile string
var listenaddr string
var passfile string
var runmode string
var logger *sutils.Logger

func init() {
	var logfile string
	var loglevel string

	flag.StringVar(&runmode, "mode", "", "client/server mode")
	flag.StringVar(&cipher, "cipher", "aes", "aes/des/tripledes/rc4")
	flag.StringVar(&keyfile, "keyfile", "", "key and iv file")
	flag.StringVar(&listenaddr, "listen", ":5233", "listen address")
	flag.StringVar(&passfile, "passfile", "", "password file")

	flag.StringVar(&logfile, "logfile", "", "log file")
	flag.StringVar(&loglevel, "loglevel", "WARNING", "log level")
	flag.Parse()

	lv, err := sutils.GetLevelByName(loglevel)
	if err != nil { log.Fatal(err.Error()) }
	err = sutils.SetupLog(logfile, lv, 16)
	if err != nil { log.Fatal(err.Error()) }

	logger = sutils.NewLogger("goproxy")
}

var cryptWrapper func (net.Conn) (net.Conn, error) = nil

func run_client () {
	// need --listenaddr serveraddr
	var err error
	var serveraddr string

	if cryptWrapper == nil {
		sutils.Warning("client mode without keyfile")
	}

	if len(flag.Args()) < 1 {
		log.Fatal("args not enough")
	}
	serveraddr = flag.Args()[0]

	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		var dstconn net.Conn
		defer conn.Close()

		sutils.Debug("connection comein")
		tcpAddr, err := net.ResolveTCPAddr("tcp", serveraddr)
		if err != nil { return }
		dstconn, err = net.DialTCP("tcp", nil, tcpAddr)
		if err != nil { return }

		if cryptWrapper != nil {
			dstconn, err = cryptWrapper(dstconn)
			if err != nil { return }
		}
		defer dstconn.Close()

		go func () {
			defer conn.Close()
			defer dstconn.Close()
			io.Copy(conn, dstconn)
		}()
		io.Copy(dstconn, conn)
		return
	})
	if err != nil { sutils.Err(err) }
}

func run_server () {
	// need --passfile --listenaddr
	var err error
		
	ap := socks.NewSockServer()
	if len(passfile) > 0 { ap.LoadFile(passfile) }
	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		if cryptWrapper != nil {
			conn, err = cryptWrapper(conn)
			if err != nil {
				logger.Err("encrypt failed,", err)
				return
			}
		}

		defer conn.Close()
		dstconn, err := ap.Handler(conn)
		if err != nil { return }
		defer dstconn.Close()

		go func () {
			defer conn.Close()
			defer dstconn.Close()
			io.Copy(conn, dstconn)
		}()
		io.Copy(dstconn, conn)
		return
	})
	if err != nil {
		log.Println(err.Error())
	}
	return
}

func main() {
	var err error

	if len(keyfile) > 0 {
		cryptWrapper, err = cryptconn.NewCryptWrapper(cipher, keyfile)
		if err != nil {
			log.Fatal("crypto not work, cipher or keyfile wrong.")
		}
	}

	switch runmode {
	case "client":
		sutils.Info("client mode")
		run_client()
	case "server":
		sutils.Info("server mode")
		run_server()
	}
}