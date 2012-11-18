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
	var buf bytes.Buffer
	var b []byte
	b = make([]byte, 10)

	buf.Write(ar[:])

	n, err := buf.Read(b)
	fmt.Println(b[:n], err)

	n, err = buf.Read(b)
	fmt.Println(b[:n], err)

	buf.Write(ar[:])

	n, err = buf.Read(b)
	fmt.Println(b[:n], err)
}