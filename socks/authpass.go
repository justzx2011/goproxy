package socks

import (
	"bufio"
	"errors"
	"log"
	"net"
	"io"
	"os"
	"strings"
	"../sutils"
)

type AuthPassword struct {
	userpass map[string]string
}

func NewAuthPassword() (ap *AuthPassword) {
	ap = new(AuthPassword)
	ap.userpass = make(map[string]string)
	return
}

func (ap *AuthPassword) Clean() {
	ap.userpass = make(map[string]string)
}

func (ap *AuthPassword) SetPassword(user string, password string) {
	ap.userpass[user] = password
	return
}

func (ap *AuthPassword) LoadFile(filepath string) (err error) {
	file, err := os.Open(filepath)
	if err != nil { return }
	defer file.Close()

	sutils.ReadLines(file, func (line string) (err error){
		p := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if err != nil { return }
		if len(p) != 2 {
			log.Fatal("password file format wrong")
		}

		ap.SetPassword(p[0], p[1])
		return
	})

	return
}

func (ap *AuthPassword) SelectMethod(methods []byte) (method byte) {
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

func (ap *AuthPassword) Handler(conn net.Conn) (err error) {
	defer conn.Close()

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
		p, ok := ap.userpass[user]
		log.Println(user, password, ap.userpass)
		if !ok || p != password {
			SendAuthResult(writer, 0x01)
			return
		}
		err = SendAuthResult(writer, 0x00)
		if err != nil { return }
	case 0xff:
		return errors.New("auth method not supported")
	}
	if DEBUG { log.Println("handshark ok") }

	var dstaddr net.TCPAddr
	dstaddr, err = GetConnect(reader)
	if err != nil { return }
	if DEBUG { log.Println("dst:", dstaddr.String()) }

	var dstconn *net.TCPConn
	dstconn, err = net.DialTCP("tcp4", nil, &dstaddr)
	if err != nil {
		SendResponse(writer, 0x04)
		return
	}
	SendResponse(writer, 0x00)

	go func () {
		io.Copy(conn, dstconn)
	}()
	io.Copy(dstconn, conn)
	return
}