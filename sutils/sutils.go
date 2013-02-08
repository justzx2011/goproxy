package sutils

import (
	"io"
	"bufio"
)

func ReadLines(r io.Reader, f func(string) (error)) (err error) {
	var line string

	reader := bufio.NewReader(r)
	for {
		line, err = reader.ReadString('\n')
		switch err {
		case io.EOF:
			if len(line) == 0 { return nil }
		case nil:
		default: return
		}
		err = f(line)
		if err != nil { return err }
	}
	return
}
