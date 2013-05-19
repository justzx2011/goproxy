package src

import (
	"bufio"
	"errors"
	"flag"
	"net"
	"net/http"
	"strconv"
	"strings"
	"../sutils"
)

func socks_handler(conn net.Conn) (srcconn net.Conn, dstconn net.Conn, err error) {
	sutils.Debug("connection comein")
	srcconn = conn

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	methods, err := GetHandshake(reader)
	if err != nil { return }

	var method byte = 0xff
	for _, m := range methods {
		if m == 0 { method = 0 }
	}
	SendHandshakeResponse(writer, method)
	if method == 0xff { return nil, nil, errors.New("auth method wrong") }
	sutils.Debug("handshark ok")

	hostname, port, err := GetConnect(reader)
	if err != nil {
		// general SOCKS server failure
		SendConnectResponse(writer, 0x01)
		return
	}
	sutils.Debug("dst:", hostname, port)

	dstconn, err = dail(hostname, port)
	if err != nil {
		// Connection refused
		SendConnectResponse(writer, 0x05)
		return
	}

	SendConnectResponse(writer, 0x00)
	return
}

func RunClient () {
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

		sutils.CopyLink(srcconn, dstconn)
		return
	})
	if err != nil { sutils.Err(err) }
}

var tspt http.Transport

type Proxy struct {}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sutils.Info(r.Method, r.URL)

	if r.Method == "CONNECT" {
		p.Connect(w, r)
		return
	}

	r.RequestURI = ""
	r.Header.Del("Accept-Encoding")
	r.Header.Del("Proxy-Connection")
	r.Header.Del("Connection")

	resp, err := tspt.RoundTrip(r)
	if err != nil {
		sutils.Err(err)
		return
	}
	defer resp.Body.Close()

	resp.Header.Del("Content-Length")
	for k, vv := range resp.Header {
		for _, v := range vv { w.Header().Add(k, v) }
	}
	w.WriteHeader(resp.StatusCode)
	_, err = sutils.CoreCopy(w, resp.Body)
	if err != nil {
		sutils.Err(err)
		return
	}
	return
}

func (p *Proxy) Connect(w http.ResponseWriter, r *http.Request) {
	hij, ok := w.(http.Hijacker)
	if !ok {
		sutils.Err("httpserver does not support hijacking")
		return
	}
	srcconn, _, err := hij.Hijack()
	if err != nil {
		sutils.Err("Cannot hijack connection ", err)
		return
	}
	defer srcconn.Close()

	host := r.URL.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	dstconn, err := dial_conn("tcp", host)
	if err != nil {
		sutils.Err(err)
		srcconn.Write([]byte("HTTP/1.0 502 OK\r\n\r\n"))
		return
	}
	defer dstconn.Close()
	srcconn.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))

	sutils.CopyLink(srcconn, dstconn)
	return
}

func dial_conn(network, addr string) (c net.Conn, err error) {
	addrs := strings.Split(addr, ":")
	hostname := addrs[0]
	port, err := strconv.Atoi(addrs[1])
	if err != nil { return }
	return dail(hostname, uint16(port))
}

func RunHttproxy() {
	if cryptWrapper == nil {
		sutils.Warning("client mode without keyfile")
	}

	if len(flag.Args()) < 1 {
		panic("args not enough")
	}
	serveraddr = flag.Args()[0]

	init_dail()

	tspt = http.Transport{Dial: dial_conn}
	http.ListenAndServe(listenaddr, &Proxy{})
}
