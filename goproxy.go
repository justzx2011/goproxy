package main

import (
	"flag"
	// "fmt"
	// "io"
	"log"
	// "math/rand"
	"net"
	// "os"
	"./socks"
	"./sutils"
	"./cryptconn"
	"./tunnel"
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

	flag.StringVar(&runmode, "mode", "", "udpcli/udpsrv/client/server mode")
	flag.StringVar(&cipher, "cipher", "aes", "aes des tripledes rc4")
	flag.StringVar(&keyfile, "keyfile", "", "key and iv file")
	flag.StringVar(&listenaddr, "listen", ":8899", "listen address")
	flag.StringVar(&passfile, "passfile", "", "password file")

	flag.StringVar(&logfile, "logfile", "", "log file")
	flag.StringVar(&loglevel, "loglevel", "WARNING", "log level")
	flag.Parse()

	lv, err := sutils.GetLevelByName(loglevel)
	if err != nil { log.Fatal(err.Error()) }
	err = sutils.SetupLog(logfile, lv, 16)
	if err != nil { log.Fatal(err.Error()) }

	logger = sutils.NewLogger("goproxy")

	// rand.Seed(1)
}

var cryptWrapper func (net.Conn) (net.Conn, error) = nil

func run_udpcli () {
	// need --listenaddr serveraddr
	var err error
	var serveraddr string

	// if len(keyfile) == 0 {
	// 	log.Println("WARN: client mode without keyfile")
	// }
	
	if len(flag.Args()) < 1 {
		log.Fatal("args not enough")
	}
	serveraddr = flag.Args()[0]

	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		var dstconn net.Conn
		defer conn.Close()

		dstconn, err = tunnel.DialTunnel(serveraddr)
		if err != nil { return }
		defer dstconn.Close()

		// if f != nil {
		// 	dstconn, err = f(dstconn)
		// 	if err != nil { return }
		// }

		go func () {
			defer conn.Close()
			defer dstconn.Close()
			tunnel.Copy(conn, dstconn)
		}()
		tunnel.Copy(dstconn, conn)
		return
	})
	if err != nil {
		log.Println(err.Error())
	}
}

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
			tunnel.Copy(conn, dstconn)
		}()
		tunnel.Copy(dstconn, conn)
		return
	})
	if err != nil { sutils.Err(err) }
}

func run_udpsrv () {
	// need --passfile --listenaddr
	var err error
		
	ap := socks.NewSockServer()
	if len(passfile) > 0 { ap.LoadFile(passfile) }
	err = tunnel.UdpServer(listenaddr, func (conn net.Conn) (err error) {
		// if f != nil {
		// 	conn, err = f(conn)
		// 	if err != nil { return }
		// }

		defer conn.Close()
		dstconn, err := ap.Handler(conn)
		if err != nil { return }
		defer dstconn.Close()

		go func () {
			defer conn.Close()
			defer dstconn.Close()
			tunnel.Copy(conn, dstconn)
		}()
		tunnel.Copy(dstconn, conn)
		return
	})
	if err != nil {
		log.Println(err.Error())
	}
	return

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
			tunnel.Copy(conn, dstconn)
		}()
		tunnel.Copy(dstconn, conn)
		return
	})
	if err != nil {
		log.Println(err.Error())
	}
	return
}

func main() {
	// with --mode [--keyfile] [--cipher]
	var err error

	if len(keyfile) > 0 {
		cryptWrapper, err = cryptconn.NewCryptWrapper(cipher, keyfile)
		if err != nil {
			log.Fatal("crypto not work, cipher or keyfile wrong.")
		}
	}

	switch runmode {
	case "udpcli":
		sutils.Info("udp client mode")
		run_udpcli()
	case "client":
		sutils.Info("client mode")
		run_client()
	case "udpsrv":
		sutils.Info("udp server mode")
		run_udpsrv()
	case "server":
		sutils.Info("server mode")
		run_server()
	}
}