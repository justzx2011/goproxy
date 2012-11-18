package main

import (
	"bytes"
	"fmt"
	// "log"
	// "net"
	// "./sutils"
)

func main () {
	ar := [4]byte{0x01, 0x02, 0x03, 0x04}
	buf := bytes.NewBuffer(ar[:])

	b := buf.Bytes()
	b[0] = 0x08

	fmt.Println(ar)
}