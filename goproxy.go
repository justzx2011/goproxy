package main

import (
	"flag"
	"log"
	"net"
	"./sutils"
	"./cryptconn"
)

var cipher string
var keyfile string
var listenaddr string
var passfile string
var runmode string
var logger *sutils.Logger

func init() {
	var logfile string
	var loglevel string

	flag.StringVar(&runmode, "mode", "", "client/server mode")
	flag.StringVar(&cipher, "cipher", "aes", "aes/des/tripledes/rc4")
	flag.StringVar(&keyfile, "keyfile", "", "key and iv file")
	flag.StringVar(&listenaddr, "listen", ":5233", "listen address")
	flag.StringVar(&passfile, "passfile", "", "password file")

	flag.StringVar(&logfile, "logfile", "", "log file")
	flag.StringVar(&loglevel, "loglevel", "WARNING", "log level")
	flag.Parse()

	lv, err := sutils.GetLevelByName(loglevel)
	if err != nil { log.Fatal(err.Error()) }
	err = sutils.SetupLog(logfile, lv, 16)
	if err != nil { log.Fatal(err.Error()) }

	logger = sutils.NewLogger("goproxy")
}

var cryptWrapper func (net.Conn) (net.Conn, error) = nil

func main() {
	var err error

	if len(keyfile) > 0 {
		cryptWrapper, err = cryptconn.NewCryptWrapper(cipher, keyfile)
		if err != nil {
			log.Fatal("crypto not work, cipher or keyfile wrong.")
		}
	}

	switch runmode {
	case "client":
		sutils.Info("client mode")
		run_client()
	case "server":
		sutils.Info("server mode")
		run_server()
	}
}