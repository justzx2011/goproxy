package main

import (
	"flag"
	"net"
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