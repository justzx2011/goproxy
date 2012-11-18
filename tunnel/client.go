package tunnel

import (
	"bufio"
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
	var buf *bufio.Writer
	var db *SendBlock

	for {
		db, ok = <- c.t.c_send
		if !ok { break }

		buf = bufio.NewWriterSize(c.conn, 2*SMSS)
		err = db.pkt.WriteTo(buf)
		if _, ok := err.(*net.OpError); ok {
			break
		}
		err = buf.Flush()
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
	var rb *RecvBlock

	for {
		rb = get_recvblock()
		rb.n, err = c.conn.Read(rb.buf[:])
		if _, ok := err.(*net.OpError); ok {
			break
		}
		if err != nil {
			logcli.Err(err)
			break
		}

		c.t.c_recv <- rb
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
	t.c_send = make(chan *SendBlock, 1)
	// t.c_sendfree = make(chan *SendBlock, 1)
	t.onclose = func () {
		logcli.Info("close tunnel", localaddr)
		err := conn.Close()
		if err != nil { logcli.Err(err) }
	}

	c := &Client{t, conn, name}
	go c.sender()
	go c.recver()

	t.c_event <- EV_CONNECT
	<- t.c_connect
	logcli.Info("create tunnel", localaddr)
	return NewTunnelConn(t), nil
}
