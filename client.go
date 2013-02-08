package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"compress/gzip"
	"io"
	"net"
	"os"
	"strings"
	"./dns"
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

var blacklist []net.IPNet

func readlist() (err error) {
	var f io.Reader
	f, err = os.Open(blackfile)
	if err != nil { return }
	if strings.HasSuffix(blackfile, ".gz") {
		f, err = gzip.NewReader(f)
		if err != nil { return }
	}
	err = sutils.ReadLines(f, func (line string) (err error){
		addrs := strings.Split(line, " ")
		ipnet := net.IPNet{net.ParseIP(addrs[0]), net.IPMask(net.ParseIP(addrs[0]))}
		blacklist = append(blacklist, ipnet)
		return
	})
	if err != nil { return }
	sutils.Info("blacklist loaded,", len(blacklist), "record(s).")
	return
}

func list_contain(ipnetlist []net.IPNet, ip net.IP) (bool) {
	for _, ipnet := range ipnetlist {
		if ipnet.Contains(ip) { return true }
	}
	return false
}

func select_connfunc(hostname string, port uint16) (connfunc func (string, uint16) (net.Conn, error), err error) {
	if blacklist == nil { return connect_qsocks, nil }
	addrs, err := dns.LookupIP(hostname)
	if err != nil { return }
	switch {
	case list_contain(blacklist, addrs[0]):
		sutils.Debug("ip", addrs[0], "in black list.")
		return connect_direct, nil
	}
	return connect_qsocks, nil
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
		// general SOCKS server failure
		socks.SendConnectResponse(writer, 0x01)
		return
	}
	sutils.Debug("dst:", hostname, port)

	connfunc, err := select_connfunc(hostname, port)
	if err != nil {
		// Address type not supported
		socks.SendConnectResponse(writer, 0x08)
		return nil, nil, errors.New("no conn function can be used")
	}
	dstconn, err = connfunc(hostname, port)
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

	if blackfile != "" {
		err := readlist()
		if err != nil { panic(err.Error()) }
	}
	loaddns()

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
