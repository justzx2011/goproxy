package main

import (
	"bufio"
	"errors"
	"flag"
	"net"
	"./socks"
	"./sutils"
)

func socks_handler(conn net.Conn) (srcconn net.Conn, dstconn net.Conn, err error) {
	sutils.Debug("connection comein")
	srcconn = conn

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	methods, err := socks.GetHandshake(reader)
	if err != nil { return }

	var method byte = 0xff
	for _, m := range methods {
		if m == 0 { method = 0 }
	}
	socks.SendHandshakeResponse(writer, method)
	if method == 0xff { return nil, nil, errors.New("auth method wrong") }
	sutils.Debug("handshark ok")

	hostname, port, err := socks.GetConnect(reader)
	if err != nil {
		// general SOCKS server failure
		socks.SendConnectResponse(writer, 0x01)
		return
	}
	sutils.Debug("dst:", hostname, port)

	dstconn, err = dail(hostname, port)
	if err != nil {
		// Connection refused
		socks.SendConnectResponse(writer, 0x05)
		return
	}

	socks.SendConnectResponse(writer, 0x00)
	return
}

func run_client () {
	var err error

	if cryptWrapper == nil {
		sutils.Warning("client mode without keyfile")
	}

	if len(flag.Args()) < 1 {
		panic("args not enough")
	}
	serveraddr = flag.Args()[0]

	init_dail()

	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		defer conn.Close()
		srcconn, dstconn, err := socks_handler(conn)
		if err != nil { return }

		copylink(srcconn, dstconn)
		return
	})
	if err != nil { sutils.Err(err) }
}
