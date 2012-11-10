package sutils

import (
	"io"
	"bufio"
	"errors"
)

func ReadLines(r io.Reader, f func(string) (error)) (err error) {
	var line string

	reader := bufio.NewReader(r)
	line, _ = reader.ReadString('\n')
	for len(line) != 0 {
		err = f(line)
		if err != nil { return err }
		line, _ = reader.ReadString('\n')
	}
	return
}

func ReadBytes(r *bufio.Reader, c int) (b []byte, err error) {
	var n int
	b = make([]byte, c)
	n, err = r.Read(b)
	if n != c { err = errors.New("read bytes dismatch") }
	return 
}
