package src

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"../sutils"
)

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

	username, password, err = GetAuth(conn)
	if err != nil { return }

	if userpass != nil {
		password1, ok := userpass[username]
		if !ok || (password != password1) {
			SendResponse(conn, 0x01)
			return fmt.Errorf("failed with auth: %s:%s", username, password)
		}
	}
	sutils.Debug("qsocks auth passed")

	req, err := GetReq(conn)
	if err != nil { return }
	switch req {
	case REQ_CONN:
		var hostname string
		var port uint16
		hostname, port, err = GetConn(conn)
		if err != nil { return }
		sutils.Debug("try connect to", hostname, port)
		var dstconn net.Conn
		dstconn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
		if err != nil { return }
		SendResponse(conn, 0)
		sutils.CopyLink(conn, dstconn)
		return
	case REQ_DNS:
		SendResponse(conn, 0xff)
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