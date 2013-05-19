package src

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

func SendHandshakeResponse(writer *bufio.Writer, status byte) (err error) {
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

func GetConnect(reader *bufio.Reader) (hostname string, port uint16, err error) {
	var c byte

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
		ip := net.IP{}
		_, err = io.ReadFull(reader, ip)
		if err != nil { return }
		hostname = ip.String()
	case 0x03: // DOMAINNAME
		sutils.Debug("socks with domain")
		hostname, err = readString(reader)
		if err != nil { return }
	case 0x04: // IP V6 address
		err = errors.New("ipv6 not support yet")
		return 
	}

	err = binary.Read(reader, binary.BigEndian, &port)
	return
}

func SendConnectResponse(writer *bufio.Writer, res byte) (err error) {
	var buf []byte = []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	var n int

	buf[1] = res
	n, err = writer.Write(buf)
	if n != len(buf) { return errors.New("send buffer full") }
	return writer.Flush()
}