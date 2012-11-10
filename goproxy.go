package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"./socks"
	"./sutils"
	"./secconn"
)

var cipher string
var keyfile string
var listenaddr string
var passfile string
var runmode string

func Usage() {
	fmt.Printf("Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
}

func init() {
	// flag.Usage = Usage
	flag.StringVar(&runmode, "mode", "", "client/server mode")
	flag.StringVar(&cipher, "cipher", "aes", "aes des tripledes rc4")
	flag.StringVar(&keyfile, "keyfile", "", "key and iv file")
	flag.StringVar(&listenaddr, "listen", "", "listen address")
	flag.StringVar(&passfile, "passfile", "", "password file")
	flag.Parse()

	if len(listenaddr) == 0 {
		listenaddr = ":1080"
		if runmode == "server" && len(keyfile) != 0 {
			listenaddr = ":8899"
		}
	}
}

var f func (net.Conn) (net.Conn, error) = nil

func run_client () {
	// need --listenaddr serveraddr
	var err error
	var serveraddr string

	if len(keyfile) == 0 {
		log.Println("WARN: client mode without keyfile")
	}

	if len(flag.Args()) < 1 {
		log.Fatal("args not enough")
	}
	serveraddr = flag.Args()[0]

	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		var dstconn net.Conn
		defer conn.Close()
		tcpAddr, err := net.ResolveTCPAddr("tcp4", serveraddr)
		if err != nil { return }
		dstconn, err = net.DialTCP("tcp4", nil, tcpAddr)
		if err != nil { return }
		defer dstconn.Close()

		if f != nil {
			dstconn, err = f(dstconn)
			if err != nil { return }
		}

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
}

func run_server () {
	// need --passfile --listenaddr
	var err error
		
	ap := socks.NewAuthPassword()
	if len(passfile) > 0 { ap.LoadFile(passfile) }
	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		if f != nil {
			conn, err = f(conn)
			if err != nil { return }
		}
		return ap.Handler(conn)
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
		f, err = secconn.NewSecConn(cipher, keyfile)
		if err != nil {
			log.Fatal("crypto not work, cipher or keyfile wrong.")
		}
	}

	switch runmode {
	case "client":
		run_client()
	case "server":
		run_server()
	default:
		Usage()
	}
}