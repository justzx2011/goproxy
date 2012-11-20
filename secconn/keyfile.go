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

func NewSecConn(method string, keyfile string) (f func (net.Conn) (net.Conn, error), err error) {
	var g func(net.Conn, []byte, []byte) (net.Conn, error)
	var keylen int
	var ivlen int

	switch(method){
	case "aes":
		keylen = 16
		ivlen = 16
		g = NewAesConn
	case "des":
		keylen = 16
		ivlen = 8
		g = NewDesConn
	case "tripledes":
		keylen = 16
		ivlen = 8
		g = NewTripleDesConn
	case "rc4":
		keylen = 16
		ivlen = 0
		g = NewRC4Conn
	}

	var key []byte
	var iv []byte
	key, iv, err = ReadKey(keyfile, keylen, ivlen)
	if err != nil { return }
	return func(conn net.Conn) (sc net.Conn, err error) {
		return g(conn, key, iv)
	}, nil
}

func NewAesConn(conn net.Conn, key []byte, iv []byte) (sc net.Conn, err error) {
	block, err := aes.NewCipher(key)
	if err != nil { return }
	in := cipher.NewCFBDecrypter(block, iv)
	out := cipher.NewCFBEncrypter(block, iv)
	return SecConn{conn.(*net.TCPConn), in, out}, nil
}

func NewDesConn(conn net.Conn, key []byte, iv []byte) (sc net.Conn, err error) {
	block, err := des.NewCipher(key)
	if err != nil { return }
	in := cipher.NewCFBDecrypter(block, iv)
	out := cipher.NewCFBEncrypter(block, iv)
	return SecConn{conn.(*net.TCPConn), in, out}, nil
}

func NewTripleDesConn(conn net.Conn, key []byte, iv []byte) (sc net.Conn, err error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil { return }
	in := cipher.NewCFBDecrypter(block, iv)
	out := cipher.NewCFBEncrypter(block, iv)
	return SecConn{conn.(*net.TCPConn), in, out}, nil
}

func NewRC4Conn(conn net.Conn, key []byte, iv []byte) (sc net.Conn, err error) {
	in, err := rc4.NewCipher(key)
	if err != nil { return }
	out, err := rc4.NewCipher(key)
	if err != nil { return }
	return SecConn{conn.(*net.TCPConn), in, out}, nil
}