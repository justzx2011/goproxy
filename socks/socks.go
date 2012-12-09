package socks

import (
	"bufio"
	"errors"
	"encoding/binary"
	"io"
	"net"
	"../sutils"
)

func readLeadByte(reader io.Reader) (b []byte, err error) {
	var c [1]byte

	n, err := reader.Read(c[:])
	if err != nil { return }
	if n < 1 { return nil, io.EOF }

	b = make([]byte, int(c[0]))
	_, err = io.ReadFull(reader, b)
	return
}

func readString(reader io.Reader) (s string, err error) {
	b, err := readLeadByte(reader)
	if err != nil { return }
	return string(b), nil
}

func GetHandshake(reader *bufio.Reader) (methods []byte, err error) {
	var c byte

	c, err = reader.ReadByte()
	if err != nil { return }
	if c != 0x05 {
		return nil, errors.New("protocol error")
	}

	methods, err = readLeadByte(reader)
	return
}

func SendHandshake(writer *bufio.Writer, status byte) (err error) {
	_, err = writer.Write([]byte{0x05, status})
	if err != nil { return }
	return writer.Flush()
}

func GetUserPass(reader *bufio.Reader) (user string, password string, err error) {
	c, err := reader.ReadByte()
	if err != nil { return }
	if c != 0x01 {
		err = errors.New("Auth Packet Error")
		return
	}

	user, err = readString(reader)
	if err != nil { return }
	password, err = readString(reader)
	return
}

func SendAuthResult(writer *bufio.Writer, status byte) (err error) {
	var buf []byte = []byte{0x01, 0x00}

	buf[1] = status
	n, err := writer.Write(buf)
	if n != len(buf) { return errors.New("send buffer full") }
	return writer.Flush()
}

func GetConnect(reader *bufio.Reader) (addr net.TCPAddr, err error) {
	var c byte
	var n int
	var port uint16

	buf := make([]byte, 3)
	_, err = io.ReadFull(reader, buf)
	if err != nil { return }
	if buf[0] != 0x05 || buf[1] != 0x01 || buf[2] != 0x00 {
		err = errors.New("connect packet wrong format")
		return
	}

	c, err = reader.ReadByte()
	if err != nil { return }

	switch c {
	case 0x01: // IP V4 address
		sutils.Debug("socks with ipaddr")
		n, err = reader.Read(addr.IP[:4])
		if err != nil { return }
		if n != 4 { return addr, errors.New("ipaddr v4 length dismatch") }
	case 0x03: // DOMAINNAME
		sutils.Debug("socks with domain")
		var ips []net.IP
		var s string

		s, err = readString(reader)
		if err != nil { return }
		ips, err = net.LookupIP(s)
		if err != nil { return }
		for _, ip := range ips {
			if ip.To4() != nil {
				addr.IP = ip
				break
			}
		}
	case 0x04: // IP V6 address
		return addr, errors.New("ipv6 not support yet")
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
	if n != len(buf) { return errors.New("send buffer full") }
	return writer.Flush()
}