package main

import (
	"flag"
	"net"
	"net/http"
	"strings"
	"./dns"
	"./src"
	"./sutils"
	"./cryptconn"
)

var cipher string
var keyfile string
var listenaddr string
var username string
var password string
var passfile string
var blackfile string
var runmode string
var logger *sutils.Logger

func init() {
	var logfile string
	var loglevel string

	flag.StringVar(&runmode, "mode", "", "server/client/httproxy mode")
	flag.StringVar(&cipher, "cipher", "aes", "aes/des/tripledes/rc4")
	flag.StringVar(&keyfile, "keyfile", "", "key and iv file")
	flag.StringVar(&listenaddr, "listen", ":5233", "listen address")
	flag.StringVar(&username, "username", "", "username for connect")
	flag.StringVar(&password, "password", "", "password for connect")
	flag.StringVar(&passfile, "passfile", "", "password file")
	flag.StringVar(&blackfile, "black", "", "blacklist file")

	flag.StringVar(&logfile, "logfile", "", "log file")
	flag.StringVar(&loglevel, "loglevel", "WARNING", "log level")
	flag.Parse()

	lv, err := sutils.GetLevelByName(loglevel)
	if err != nil { panic(err.Error()) }
	err = sutils.SetupLog(logfile, lv, 16)
	if err != nil { panic(err.Error()) }

	logger = sutils.NewLogger("goproxy")
}

var cryptWrapper func (net.Conn) (net.Conn, error) = nil

func run_server () {
	var err error

	if passfile != "" {
		err = src.LoadPassfile(passfile)
		if err != nil { panic(err.Error()) }
	}
		
	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		defer conn.Close()
		err = src.QsocksHandler(conn)
		if err != nil { sutils.Err(err) }
		return nil
	})
	if err != nil { sutils.Err(err) }
}

func run_client () {
	var err error

	if cryptWrapper == nil {
		sutils.Warning("client mode without keyfile")
	}

	if len(flag.Args()) < 1 {
		panic("args not enough")
	}
	serveraddr := flag.Args()[0]

	err = dns.LoadConfig("resolv.conf")
	if err != nil {
		err = dns.LoadConfig("/etc/goproxy/resolv.conf")
		if err != nil { panic(err.Error()) }
	}

	src.InitDail(blackfile, serveraddr, cryptWrapper, username, password)

	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		defer conn.Close()
		srcconn, dstconn, err := src.SocksHandler(conn)
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
	dstconn, err := src.DialConn("tcp", host)
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

func run_httproxy() {
	if cryptWrapper == nil {
		sutils.Warning("client mode without keyfile")
	}

	if len(flag.Args()) < 1 {
		panic("args not enough")
	}
	serveraddr := flag.Args()[0]

	err := dns.LoadConfig("resolv.conf")
	if err != nil {
		err = dns.LoadConfig("/etc/goproxy/resolv.conf")
		if err != nil { panic(err.Error()) }
	}

	src.InitDail(blackfile, serveraddr, cryptWrapper, username, password)

	tspt = http.Transport{Dial: src.DialConn}
	http.ListenAndServe(listenaddr, &Proxy{})
}

func main() {
	var err error

	if len(keyfile) > 0 {
		cryptWrapper, err = cryptconn.NewCryptWrapper(cipher, keyfile)
		if err != nil {
			sutils.Err("crypto not work, cipher or keyfile wrong.")
			return
		}
	}

	switch runmode {
	case "server":
		sutils.Info("server mode")
		run_server()
	case "client":
		sutils.Info("client mode")
		run_client()
	case "httproxy":
		sutils.Info("httproxy mode")
		run_httproxy()
	}
}