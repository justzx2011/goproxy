package main

import (
	"flag"
	"net"
	"net/http"
	"strconv"
	"strings"
	"./sutils"
)

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
	_, err = coreCopy(w, resp.Body)
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

	copylink(srcconn, dstconn)
	return
}

func dial_conn(network, addr string) (c net.Conn, err error) {
	addrs := strings.Split(addr, ":")
	hostname := addrs[0]
	port, err := strconv.Atoi(addrs[1])
	if err != nil { return }
	return dail(hostname, uint16(port))
}

func run_httproxy() {
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
