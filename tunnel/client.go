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
	var n int
	var db *SendBlock

	for {
		db = <- c.t.c_send

		n, err = db.pkt.Pack()
		if err != nil {
			logcli.Err(err)
			continue
		}

		_, err = c.conn.Write(db.pkt.buf[:n])
		if _, ok := err.(*net.OpError); ok {
			continue
			// break
		}
		if err != nil {
			logcli.Err(err)
			continue
			// break
		}
	}
}

func (c *Client) recver () {
	var err error
	var n int
	var pkt *Packet

	for {
		pkt = get_packet()
		n, err = c.conn.Read(pkt.buf[:])
		if _, ok := err.(*net.OpError); ok {
			put_packet(pkt)
			// break
			continue
		}
		if err != nil {
			put_packet(pkt)
			logcli.Err(err)
			continue
			// break
		}

		err = pkt.Unpack(n)
		if err != nil { continue }
		c.t.c_recv <- pkt
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
