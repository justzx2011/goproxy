package main

import (
	// "bytes"
	// "errors"
	"fmt"
	// "encoding/binary"
	// "io"
	// "log"
	// "net"
	// "time"
	// "reflect"
	// "./sutils"
	// "./tunnel"
)

func main () {
	var c chan uint8
	c = nil
	b := <- c
	fmt.Println(b)
}