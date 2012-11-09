package secconn

import (
	"bufio"
	"crypto/cipher"
	"crypto/aes"
	"crypto/des"
	"crypto/rc4"
	"net"
	"os"
	"../sutils"
)

func ReadKey(keyfile string, keysize int, ivsize int) (key []byte, iv []byte, err error) {
	file, err := os.Open(keyfile)
	if err != nil { return }
	defer file.Close()

	reader := bufio.NewReader(file)
	key, err = sutils.ReadBytes(reader, keysize)
	if err != nil { return }
	iv, err = sutils.ReadBytes(reader, keysize)
	return
}

func NewSecConn(method string, keyfile string) (f func (conn net.Conn) (sc *SecConn, err error), err error) {
	var key []byte
	var iv []byte

	switch(method){
	case "aes":
		key, iv, err = ReadKey(keyfile, 16, 16)
		if err != nil { return }
		f = func(conn net.Conn) (sc *SecConn, err error) {
			block, err := aes.NewCipher(key)
			if err != nil { return }
			in := cipher.NewCFBDecrypter(block, iv)
			out := cipher.NewCFBEncrypter(block, iv)
			sc = &SecConn{conn, in, out}
			return
		}
		return
	case "des":
		key, iv, err = ReadKey(keyfile, 16, 8)
		if err != nil { return }
		f = func(conn net.Conn) (sc *SecConn, err error) {
			block, err := des.NewCipher(key)
			if err != nil { return }
			in := cipher.NewCFBDecrypter(block, iv)
			out := cipher.NewCFBEncrypter(block, iv)
			sc = &SecConn{conn, in, out}
			return
		}
		return 
	case "tripledes":
		key, iv, err = ReadKey(keyfile, 16, 8)
		if err != nil { return }
		f = func(conn net.Conn) (sc *SecConn, err error) {
			block, err := des.NewTripleDESCipher(key)
			if err != nil { return }
			in := cipher.NewCFBDecrypter(block, iv)
			out := cipher.NewCFBEncrypter(block, iv)
			sc = &SecConn{conn, in, out}
			return
		}
		return
	case "rc4":
		key, iv, err = ReadKey(keyfile, 16, 0)
		if err != nil { return }
		f = func(conn net.Conn) (sc *SecConn, err error) {
			in, err := rc4.NewCipher(key)
			if err != nil { return }
			out, err := rc4.NewCipher(key)
			if err != nil { return }
			sc = &SecConn{conn, in, out}
			return
		}
		return
	}
	return
}
