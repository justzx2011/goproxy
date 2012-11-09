package main

import (
	"flag"
	"io"
	"log"
	"net"
	"./socks"
	"./sutils"
	"./secconn"
)

var client_mode bool
var cipher string
var keyfile string
var server_mode bool
var listenaddr string
var socksaddr string
var passfile string

func init() {
	flag.BoolVar(&client_mode, "client", false, "client mode")
	flag.StringVar(&cipher, "cipher", "aes", "aes des tripledes rc4")
	flag.StringVar(&keyfile, "keyfile", "file.key", "key and iv file")
	flag.BoolVar(&server_mode, "server", false, "server mode")
	flag.StringVar(&listenaddr, "listen", ":8899", "listen address")
	flag.StringVar(&socksaddr, "socks", ":1080", "socksv5 address")
	flag.StringVar(&passfile, "passfile", "", "password file")
	flag.Parse()
}

func run_client () {
	// need --client --cipher --keyfile --socks serveraddr
	var err error
	var serveraddr string
	if len(flag.Args()) < 1 {
		log.Fatal("args not enough")
	}
	serveraddr = flag.Args()[0]

	f, err := secconn.NewSecConn(cipher, keyfile)
	if err != nil {
		log.Fatal("crypto not work, cipher or keyfile wrong.")
	}

	err = sutils.TcpServer(socksaddr, func (conn net.Conn) (err error) {
		defer conn.Close()
		tcpAddr, err := net.ResolveTCPAddr("tcp4", serveraddr)
		if err != nil { return }
		dstconn, err := net.DialTCP("tcp4", nil, tcpAddr)
		if err != nil { return }
		defer dstconn.Close()

		secdst, err := f(dstconn)
		if err != nil { return }

		go func () {
			defer conn.Close()
			defer dstconn.Close()
			io.Copy(conn, secdst)
		}()
		io.Copy(secdst, conn)
		return
	})
	if err != nil {
		log.Println(err.Error())
	}
}

func run_server () {
	// need --server --keyfile --passfile --listenaddr
	var err error
	f, err := secconn.NewSecConn(cipher, keyfile)
	if err != nil {
		log.Fatal("crypto not work, cipher or keyfile wrong.")
	}

	ap := socks.NewAuthPassword()
	if len(passfile) > 0 { ap.LoadFile(passfile) }
	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		secsrc, err := f(conn)
		if err != nil { return }
		return ap.Handler(secsrc)
	})
	if err != nil {
		log.Println(err.Error())
	}
	return
}

func run_socks () {
	// need --socks --passfile
	ap := socks.NewAuthPassword()
	if len(passfile) > 0 { ap.LoadFile(passfile) }
	err := sutils.TcpServer(socksaddr, func (conn net.Conn) (err error){
		return ap.Handler(conn)
	})
	if err != nil {
		log.Println(err.Error())
	}
}

func main() {
	if client_mode {
		run_client()
	}else if server_mode {
		run_server()
	}else{
		run_socks()
	}
}