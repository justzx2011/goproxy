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

func getString(r io.Reader) (s string, err error) {
	var size [1]byte
	_, err = r.Read(size[:])
	if err != nil { return }
	buf := make([]byte, uint8(size[0]))
	_, err = io.ReadFull(r, buf)
	if err != nil { return }
	return string(buf), nil
}


func SendRequest(conn net.Conn, username string, password string,
	hostname string, port uint16) (err error) {
	size := uint16(16 + 3 + len(username) + len(password) + len(hostname) + 2)
	buf := make([]byte, size)

	_, err = rand.Read(buf[:16])
	if err != nil { return }
	cur := buf[16:]

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

	var buf1 [16]byte
	_, err = io.ReadFull(conn, buf1[:])
	if err != nil { return }

	username, err = getString(conn)
	if err != nil { return }
	password, err = getString(conn)
	if err != nil { return }
	hostname, err = getString(conn)
	if err != nil { return }

	var buf2 [2]byte
	_, err = conn.Read(buf2[:])
	if err != nil { return }
	port = binary.BigEndian.Uint16(buf2[:])

	return
}

func SendResponse(conn net.Conn, res uint8) (err error) {
	var buf [1]byte
	buf[0] = byte(res)
	_, err = conn.Write(buf[:])
	if err != nil { return }
	return
}

func RecvResponse(conn net.Conn) (res uint8, err error) {
	var buf [1]byte
	_, err = conn.Read(buf[:])
	if err != nil { return }
	res = uint8(buf[0])
	return
}