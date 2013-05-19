package src

import (
	"bufio"
	"errors"
	"net"
	"../sutils"
)

func SocksHandler(conn net.Conn) (srcconn net.Conn, dstconn net.Conn, err error) {
	sutils.Debug("connection comein")
	srcconn = conn

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	methods, err := GetHandshake(reader)
	if err != nil { return }

	var method byte = 0xff
	for _, m := range methods {
		if m == 0 { method = 0 }
	}
	SendHandshakeResponse(writer, method)
	if method == 0xff { return nil, nil, errors.New("auth method wrong") }
	sutils.Debug("handshark ok")

	hostname, port, err := GetConnect(reader)
	if err != nil {
		// general SOCKS server failure
		SendConnectResponse(writer, 0x01)
		return
	}
	sutils.Debug("dst:", hostname, port)

	dstconn, err = Dail(hostname, port)
	if err != nil {
		// Connection refused
		SendConnectResponse(writer, 0x05)
		return
	}

	SendConnectResponse(writer, 0x00)
	return
}