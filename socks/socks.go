package socks

import (
	"bufio"
	"errors"
	"encoding/binary"
	"log"
	"net"
	"../sutils"
)

const DEBUG = false

func GetString(reader *bufio.Reader) (s string, err error) {
	var c byte

	c, err = reader.ReadByte()
	if err != nil { return }

	buf, err := sutils.ReadBytes(reader, int(c))
	if err != nil { return }
	s = string(buf)
	return
}

func GetHandshake(reader *bufio.Reader) (methods []byte, err error) {
	var c byte

	c, err = reader.ReadByte()
	if err != nil { return }
	if c != 0x05 {
		err = errors.New("protocol error")
		return
	}

	c, err = reader.ReadByte()
	if err != nil { return }

	methods, err = sutils.ReadBytes(reader, int(c))
	return
}

func SendHandshake(writer *bufio.Writer, status byte) (err error) {
	writer.Write([]byte{0x05, status})
	return writer.Flush()
}

func GetUserPass(reader *bufio.Reader) (user string, password string, err error) {
	c, err := reader.ReadByte()
	if err != nil { return }
	if c != 0x01 {
		err = errors.New("Auth Packet Error")
		return
	}

	user, err = GetString(reader)
	if err != nil { return }
	password, err = GetString(reader)
	return
}

func SendAuthResult(writer *bufio.Writer, status byte) (err error) {
	var buf []byte = []byte{0x01, 0x00}

	buf[1] = status
	n, err := writer.Write(buf)
	if n != len(buf) { err = errors.New("send buffer full") }
	writer.Flush()
	return
}

func GetConnect(reader *bufio.Reader) (addr net.TCPAddr, err error) {
	var c byte
	var n int
	var port uint16

	buf := make([]byte, 3)
	n, err = reader.Read(buf)
	if err != nil { return }
	if n != 3 {
		err = errors.New("header length dismatch")
		return
	}
	if buf[0] != 0x05 || buf[1] != 0x01 || buf[2] != 0x00 {
		err = errors.New("connect packet wrong format")
		return
	}

	c, err = reader.ReadByte()
	switch c {
	case 0x01: // IP V4 address
		if DEBUG { log.Println("socks with ipaddr") }
		n, err = reader.Read(addr.IP[:])
		if err != nil { return }
		if n != 4 {
			err = errors.New("ipaddr v4 length dismatch")
			return
		}
	case 0x03: // DOMAINNAME
		if DEBUG { log.Println("socks with domain") }
		var ips []net.IP
		c, err = reader.ReadByte()
		if err != nil { return }
		buf := make([]byte, c)
		n, err = reader.Read(buf)
		if err != nil { return }
		if n != int(c) {
			err = errors.New("domain length dismatch")
			return
		}
		ips, err = net.LookupIP(string(buf))
		if err != nil { return }
		for _, ip := range ips {
			if ip.To4() != nil {
				addr.IP = ip
				break
			}
		}
	case 0x04: // IP V6 address
		err = errors.New("ipv6 not support yet")
		return
	}

	err = binary.Read(reader, binary.BigEndian, &port)
	if err != nil { return }
	addr.Port = int(port)
	return
}

func SendResponse(writer *bufio.Writer, res byte) (err error) {
	var buf []byte = []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	var n int

	buf[1] = res
	n, err = writer.Write(buf)
	if n != len(buf) { err = errors.New("send buffer full") }
	writer.Flush()

	return 
}