package qsocks

import (
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
)

func fillString(b []byte, s string) (r []byte) {
	b[0] = byte(len(s))
	copy(b[1:], []byte(s))
	return b[len(s)+1:]
}

func getString(b []byte) (r []byte, s string) {
	c := uint8(b[0])
	return b[1+c:], string(b[1:1+c])
}

func SendRequest(conn net.Conn, username string, password string,
	hostname string, port uint16) (err error) {
	size := uint16(16 + 2 + 3 + len(username) + len(password) + len(hostname) + 2)
	buf := make([]byte, size)

	_, err = rand.Read(buf[:16])
	if err != nil { return }
	cur := buf[16:]

	binary.BigEndian.PutUint16(cur, size)
	cur = cur[2:]
	cur = fillString(cur, username)
	cur = fillString(cur, password)
	cur = fillString(cur, hostname)
	binary.BigEndian.PutUint16(cur, port)

	_, err = conn.Write(buf)
	if err != nil { return }
	return
}

func RecvRequest(conn net.Conn) (username string, password string,
	hostname string, port uint16, err error) {
	buf1 := make([]byte, 18)
	_, err = io.ReadFull(conn, buf1)
	if err != nil { return }
	size := binary.BigEndian.Uint16(buf1[16:])

	buf2 := make([]byte, size-18)
	_, err = io.ReadFull(conn, buf2)
	if err != nil { return }

	buf2, username = getString(buf2)
	buf2, password = getString(buf2)
	buf2, hostname = getString(buf2)
	port = binary.BigEndian.Uint16(buf2)

	return
}

func SendResponse(conn net.Conn, res uint8) (err error) {
	buf := make([]byte, 1)
	buf[0] = byte(res)
	_, err = conn.Write(buf)
	if err != nil { return }
	return
}

func RecvResponse(conn net.Conn) (res uint8, err error) {
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err != nil { return }
	res = uint8(buf[0])
	return
}