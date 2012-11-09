package secconn

import (
	"crypto/cipher"
	"encoding/hex"
	"log"
	"net"
	"time"
)

const DEBUG = false

type SecConn struct {
	conn net.Conn
	in cipher.Stream
	out cipher.Stream
}

func (sc SecConn) Read(b []byte) (n int, err error) {
	n, err = sc.conn.Read(b)
	if err != nil { return }
	sc.in.XORKeyStream(b[:n], b[:n])
	if DEBUG { log.Println("recv\n", hex.Dump(b[:n])) }
	return 
}

func (sc SecConn) Write(b []byte) (n int, err error) {
	if DEBUG { log.Println("send\n", hex.Dump(b)) }
	sc.out.XORKeyStream(b[:], b[:])
	n, err =  sc.conn.Write(b)
	return
}

func (sc SecConn) Close() error {
	return sc.conn.Close()
}

func (sc SecConn) LocalAddr() net.Addr {
	return sc.conn.LocalAddr()
}

func (sc SecConn) RemoteAddr() net.Addr {
	return sc.conn.RemoteAddr()
}

func (sc SecConn) SetDeadline(t time.Time) error {
	return sc.conn.SetDeadline(t)
}

func (sc SecConn) SetReadDeadline(t time.Time) error {
	return sc.conn.SetReadDeadline(t)
}

func (sc SecConn) SetWriteDeadline(t time.Time) error {
	return sc.conn.SetWriteDeadline(t)
}