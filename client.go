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

	bufAuth, err := qsocks.Auth(username, password)
	if err != nil { return }
	_, err = conn.Write(bufAuth)
	if err != nil { return }

	bufConn, err := qsocks.Conn(hostname, port)
	if err != nil { return }
	_, err = conn.Write(bufConn)
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
		addrs := strings.Split(strings.Trim(line, "\r\n "), " ")
		ipnet := net.IPNet{IP: net.ParseIP(addrs[0]), Mask: net.IPMask(net.ParseIP(addrs[1]))}
		blacklist = append(blacklist, ipnet)
		return
	})
	if err != nil { return }
	sutils.Info("blacklist loaded,", len(blacklist), "record(s).")
	return
}

func list_contain(ipnetlist []net.IPNet, ip net.IP) (bool) {
	for _, ipnet := range ipnetlist {
		if ipnet.Contains(ip) {
			sutils.Debug(ipnet, "matches")
			return true
		}
	}
	return false
}

func dail(hostname string, port uint16) (c net.Conn, err error) {
	if blacklist == nil {
		return connect_direct(hostname, port)
	}
	addr := net.ParseIP(hostname)
	if addr == nil {
		var addrs []net.IP
		// TODO: lookup?
		addrs, err = dns.LookupIP(hostname)
		if err != nil { return }
		addr = addrs[0]
	}
	switch {
	case list_contain(blacklist, addr):
		sutils.Debug("ip", addr, "in black list.")
		return connect_direct(hostname, port)
	}
	return connect_qsocks(hostname, port)
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

	if blackfile != "" {
		err := readlist()
		if err != nil { panic(err.Error()) }
	}
	loaddns()

	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		defer conn.Close()
		srcconn, dstconn, err := socks_handler(conn)
		if err != nil { return }

		copylink(srcconn, dstconn)
		return
	})
	if err != nil { sutils.Err(err) }
}
