package main

import (
	"bytes"
	"fmt"
	// "log"
	// "net"
	// "./sutils"
)

func main () {
	var buf bytes.Buffer

	ar := [4]byte{0x01, 0x02, 0x03, 0x04}
	buf.Write(ar[:])

	ar[0] = 0x08

	fmt.Println(ar, buf.Bytes())
}