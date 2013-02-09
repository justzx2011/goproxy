package main

import (
	"fmt"
	"io"
	"net"
	"./qsocks"
	"./sutils"
)

func qsocks_handler(conn net.Conn) (srcconn net.Conn, dstconn net.Conn, err error) {
	sutils.Debug("connection comein")
	srcconn = conn
	if cryptWrapper != nil {
		srcconn, err = cryptWrapper(conn)
		if err != nil {
			logger.Err("encrypt failed,", err)
			return
		}
	}

	_, _, hostname, port, err := qsocks.RecvRequest(srcconn)
	if err != nil { return }

	// TODO: check username and password
	// qsocks.SendResponse(srcconn, 0xff)
	sutils.Debug("qsocks auth passed")

	sutils.Debug("try connect to", hostname, port)
	dstconn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
	if err != nil { return }

	qsocks.SendResponse(srcconn, 0)
	return
}

func run_server () {
	var err error
		
	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		defer conn.Close()
		srcconn, dstconn, err := qsocks_handler(conn)
		if err != nil { return }
		defer dstconn.Close()

		go func () {
			defer srcconn.Close()
			defer dstconn.Close()
			io.Copy(srcconn, dstconn)
		}()
		io.Copy(dstconn, srcconn)
		return
	})
	if err != nil { sutils.Err(err) }
}