package sutils

import (
	"io"
	"bufio"
)

func ReadLines(r io.Reader, f func(string) (error)) (err error) {
	var line string
	var loop bool = true

	reader := bufio.NewReader(r)
	for loop {
		line, _ = reader.ReadString('\n')
		switch err {
		case io.EOF:
			if len(line) == 0 { return }
			loop = false
		case nil:
		default: return
		}
		err = f(line)
		if err != nil { return err }
	}
	return
}
