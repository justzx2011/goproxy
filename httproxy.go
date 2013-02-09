package main

import (
	"flag"
	"io"
	// "io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"./sutils"
)

var http_client http.Client

type Proxy struct {}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sutils.Info(r.Method, r.URL)

	if r.Method == "CONNECT" {
		p.Connect(w, r)
		return
	}

	r.RequestURI = ""
	resp, err := http_client.Do(r)
	if err != nil {
		sutils.Err(err)
		return
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	for _, c := range resp.Cookies() {
		w.Header().Add("Set-Cookie", c.Raw)
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
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

	go func () {
		defer srcconn.Close()
		defer dstconn.Close()
		io.Copy(srcconn, dstconn)
	}()
	io.Copy(dstconn, srcconn)
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

	if blackfile != "" {
		err := readlist()
		if err != nil { panic(err.Error()) }
	}
	loaddns()

	http_client = http.Client{Transport: &http.Transport{Dial: dial_conn}}

	// http.HandleFunc("/", http_handler)
	http.ListenAndServe(listenaddr, &Proxy{})
}
