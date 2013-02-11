package main

import (
	"errors"
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

func qsocks_handler(conn net.Conn) (srcconn net.Conn, dstconn net.Conn, err error) {
	sutils.Debug("connection comein")
	srcconn = conn
	if cryptWrapper != nil {
		srcconn, err = cryptWrapper(conn)
		if err != nil { return }
	}

	username, password, hostname, port, err := qsocks.RecvRequest(srcconn)
	if err != nil { return }

	if userpass != nil {
		password1, ok := userpass[username]
		if ok && (password == password1) {
			qsocks.SendResponse(srcconn, 0x01)
			return nil, nil, errors.New("failed with auth")
		}
	}
	sutils.Debug("qsocks auth passed")

	sutils.Debug("try connect to", hostname, port)
	dstconn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
	if err != nil { return }

	qsocks.SendResponse(srcconn, 0)
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
		srcconn, dstconn, err := qsocks_handler(conn)
		if err != nil {
			sutils.Err(err)
			return nil
		}

		copylink(srcconn, dstconn)
		return nil
	})
	if err != nil { sutils.Err(err) }
}