package socks

import (
	"bufio"
	"errors"
	"log"
	"net"
	"os"
	"strings"
	"../sutils"
)

type SockServer struct {
	userpass map[string]string
}

func NewSockServer() (ap *SockServer) {
	ap = new(SockServer)
	ap.userpass = make(map[string]string)
	return
}

func (ap *SockServer) Clean() {
	ap.userpass = make(map[string]string)
}

func (ap *SockServer) SetPassword(user string, password string) {
	ap.userpass[user] = password
}

func (ap *SockServer) LoadFile(filepath string) (err error) {
	file, err := os.Open(filepath)
	if err != nil { return }
	defer file.Close()

	return sutils.ReadLines(file, func (line string) (err error){
		p := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(p) != 2 {
			log.Fatal("password file format wrong")
		}

		ap.SetPassword(p[0], p[1])
		return
	})
}

func (ap *SockServer) SelectMethod(methods []byte) (method byte) {
	if len(ap.userpass) > 0 {
		method = 0x02
	}else{
		method = 0x00
	}

	for _, m := range methods {
		if method == m {
			return method
		}
	}
	return 0xff
}

func (ap *SockServer) Handler(conn net.Conn) (dstconn *net.TCPConn, err error) {
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	methods, err := GetHandshake(reader)
	if err != nil { return }

	method := ap.SelectMethod(methods)
	SendHandshake(writer, method)

	switch method {
	case 0x02:
		var user, password string
		user, password, err = GetUserPass(reader)
		if err != nil { return }

		p, ok := ap.userpass[user]
		sutils.Debug("socks5 userpass auth:", user, password, ap.userpass)
		if !ok || p != password {
			SendAuthResult(writer, 0x01)
			return
		}
		err = SendAuthResult(writer, 0x00)
		if err != nil { return }
	case 0xff:
		return nil, errors.New("auth method not supported")
	}
	sutils.Debug("handshark ok")

	var dstaddr net.TCPAddr
	dstaddr, err = GetConnect(reader)
	if err != nil { return }
	sutils.Debug("dst:", dstaddr)

	dstconn, err = net.DialTCP("tcp", nil, &dstaddr)
	if err != nil {
		sutils.Err("socks try to connect", dstaddr, "failed.")
		SendResponse(writer, 0x04)
		return
	}

	SendResponse(writer, 0x00)
	return
}