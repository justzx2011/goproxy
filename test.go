package main

import (
	// "bytes"
	// "encoding/binary"
	"log/syslog"
	// "fmt"
)

const SYSLOGADDR = "localhost:4455"

var logger *syslog.Writer

func init () {
	logger, _ = syslog.Dial("udp", SYSLOGADDR, syslog.LOG_DEBUG, "tunnel")
}

func main () {
	logger.Debug("ok")
}