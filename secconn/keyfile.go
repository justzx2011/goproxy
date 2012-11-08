package secconn

import (
	"errors"
	"os"
)

func ReadKey(keyfile string) (key []byte, iv []byte, err error) {
	var n int
	file, err := os.Open(keyfile)
	if err != nil { return }
	defer file.Close()

	key = make([]byte, 16)
	iv = make([]byte, 16)
	n, err = file.Read(key)
	if n != 16 { err = errors.New("buffer not full") }
	if err != nil { return }
	n, err = file.Read(iv)
	if n != 16 { err = errors.New("buffer not full") }
	if err != nil { return }
	return
}