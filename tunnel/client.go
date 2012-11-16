package tunnel

import (
	"fmt"
	"net"
	"strings"
	"../sutils"
)

var logcli *sutils.Logger

func init () {
	logcli = sutils.NewLogger("client")
}

type Client struct {
	t *Tunnel
	conn *net.UDPConn
	name string
}

func (c *Client) sender () {
	var err error
	var ok bool
	var db *DataBlock

	for {
		db, ok = <- c.t.c_send
		if !ok { break }
		_, err = c.conn.Write(db.buf)
		if _, ok := err.(*net.OpError); ok {
			break
		}
		if err != nil {
			logcli.Err(err)
			break
		}
	}
}

func (c *Client) recver () {
	var err error
	var n int
	var buf []byte

	for {
		buf = make([]byte, 2048)
		n, err = c.conn.Read(buf)
		if _, ok := err.(*net.OpError); ok {
			break
		}
		if err != nil {
			logcli.Err(err)
			break
		}

		c.t.c_recv <- buf[:n]
	}
}

func DialTunnel(addr string) (tc net.Conn, err error) {
	var conn *net.UDPConn
	var t *Tunnel

	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	conn, err = net.DialUDP("udp", nil, udpaddr)
	if err != nil { return }
	localaddr := conn.LocalAddr()

	name := fmt.Sprintf("%s_cli", strings.Split(localaddr.String(), ":")[1])
	t = NewTunnel(udpaddr, name)
	t.c_send = make(chan *DataBlock, 1)
	t.onclose = func () {
		logcli.Info("close tunnel", localaddr)
		conn.Close()
	}

	c := &Client{t, conn, name}
	go c.sender()
	go c.recver()

	t.c_event <- EV_CONNECT
	<- t.c_connect
	logcli.Info("create tunnel", localaddr)
	return NewTunnelConn(t), nil
}
