package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	// "io"
	"log"
	"net"
	// "os"
	"./socks"
	// "./sutils"
)

var server string
var port int
var listenaddr string

func init() {
	flag.StringVar(&server, "server", "127.0.0.1", "server name")
	flag.IntVar(&port, "port", 5233, "server port")
	flag.StringVar(&listenaddr, "listen", ":1080", "listen address")
	flag.Parse()
}

func sock_handler(conn net.Conn) (err error) {
	
}

func main() {
	sutils.TcpServer(listenaddr, sock_handler)
}
