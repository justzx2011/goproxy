package main

import (
	"flag"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"./sutils"
)

var http_client http.Client

func http_handler(w http.ResponseWriter, r *http.Request) {
	sutils.Info(r.Method, r.URL)
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
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil && err != io.EOF { panic(err) }
	w.Write(result)
}

func dial_conn(network, addr string) (c net.Conn, err error) {
	addrs := strings.Split(addr, ":")
	hostname := addrs[0]
	port, err := strconv.Atoi(addrs[1])
	if err != nil { return }
	connfunc, err := select_connfunc(hostname, uint16(port))
	if err != nil { return }
	c, err = connfunc(hostname, uint16(port))
	return
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

	http.HandleFunc("/", http_handler)
	http.ListenAndServe(listenaddr, nil)
}
