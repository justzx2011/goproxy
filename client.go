package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"./qsocks"
	"./socks"
	"./sutils"
)

var serveraddr string

func connect_qsocks(hostname string, port uint16) (conn net.Conn, err error) {
	conn, err = net.Dial("tcp", serveraddr)
	if err != nil { return }

	if cryptWrapper != nil {
		conn, err = cryptWrapper(conn)
		if err != nil { return }
	}

	err = qsocks.SendRequest(conn, "", "", hostname, port)
	if err != nil { return }
	res, err := qsocks.RecvResponse(conn)
	if err != nil { return }
	if res != 0 { return nil, fmt.Errorf("qsocks response %d", res) }
	return
}

func connect_direct(hostname string, port uint16) (conn net.Conn, err error) {
	return net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
}

func select_connfunc(hostname string, port uint16) (func (string, uint16) (net.Conn, error)) {
	return connect_qsocks
}

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
		socks.SendConnectResponse(writer, 0xff)
		return
	}
	sutils.Debug("dst:", hostname, port)

	connfunc := select_connfunc(hostname, port)
	if connfunc == nil {
		socks.SendConnectResponse(writer, 0xff)
		return nil, nil, errors.New("no conn function can be used")
	}
	dstconn, err = connfunc(hostname, port)
	if err != nil {
		socks.SendConnectResponse(writer, 0xff)
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
		log.Fatal("args not enough")
	}
	serveraddr = flag.Args()[0]

	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		defer conn.Close()
		srcconn, dstconn, err := socks_handler(conn)
		if err != nil { return }
		defer dstconn.Close()

		go func () {
			defer srcconn.Close()
			defer dstconn.Close()
			io.Copy(srcconn, dstconn)
		}()
		io.Copy(dstconn, srcconn)
		return
	})
	if err != nil { sutils.Err(err) }
}
