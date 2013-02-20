package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"./qsocks"
	"./sutils"
)

func coreCopy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := make([]byte, 1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 { written += int64(nw) }
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF { break }
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}

func copylink(src, dst io.ReadWriteCloser) {
	defer src.Close()
	defer dst.Close()
	go func () {
		defer src.Close()
		defer dst.Close()
		io.Copy(src, dst)
	}()
	io.Copy(dst, src)
}

var userpass map[string]string

func load_passfile(filename string) (err error) {
	userpass = make(map[string]string, 0)
	file, err := os.Open(filename)
	if err != nil { return }
	defer file.Close()

	sutils.ReadLines(file, func (line string) (err error){
		f := strings.SplitN(line, ":", 2)
		if len(f) < 2 { return fmt.Errorf("format wrong: %s", line) }
		userpass[strings.Trim(f[0], "\r\n ")] = strings.Trim(f[1], "\r\n ")
		return
	})
	return
}

func qsocks_handler(conn net.Conn) (err error) {
	sutils.Debug("connection comein")
	if cryptWrapper != nil {
		conn, err = cryptWrapper(conn)
		if err != nil { return }
	}

	username, password, err = qsocks.GetAuth(conn)
	if err != nil { return }

	if userpass != nil {
		password1, ok := userpass[username]
		if !ok || (password != password1) {
			qsocks.SendResponse(conn, 0x01)
			return fmt.Errorf("failed with auth: %s:%s", username, password)
		}
	}
	sutils.Debug("qsocks auth passed")

	req, err := qsocks.GetReq(conn)
	if err != nil { return }
	switch req {
	case qsocks.REQ_CONN:
		var hostname string
		var port uint16
		hostname, port, err = qsocks.GetConn(conn)
		if err != nil { return }
		sutils.Debug("try connect to", hostname, port)
		var dstconn net.Conn
		dstconn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
		if err != nil { return }
		qsocks.SendResponse(conn, 0)
		copylink(conn, dstconn)
		return
	case qsocks.REQ_DNS:
		qsocks.SendResponse(conn, 0xff)
		return errors.New("require DNS not support yet")
	}
	return
}

func run_server () {
	var err error

	if passfile != "" {
		err = load_passfile(passfile)
		if err != nil { panic(err.Error()) }
	}
		
	err = sutils.TcpServer(listenaddr, func (conn net.Conn) (err error) {
		defer conn.Close()
		err = qsocks_handler(conn)
		if err != nil { sutils.Err(err) }
		return nil
	})
	if err != nil { sutils.Err(err) }
}