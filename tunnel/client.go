package tunnel

import (
	"fmt"
	"net"
	"strings"
	"../sutils"
)

var logcli *sutils.Logger
var connlog map[string]*Tunnel

func init () {
	logcli = sutils.NewLogger("client")
	connlog = make(map[string]*Tunnel)
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
	defer func () { c.t.c_event <- EV_END }()

	for !c.t.isquit() {
		db = <- c.t.c_send

		n, err = db.pkt.Pack()
		if err != nil {
			logcli.Err(err)
			continue
		}

		_, err = c.conn.Write(db.pkt.buf[:n])
		if err != nil {
			if strings.HasSuffix(err.Error(), "use of closed network connection") {
				break
			}
			logcli.Err(err)
			continue
		}
	}
}

func (c *Client) recver () {
	var err error
	var n int
	var pkt *Packet
	defer func () { c.t.c_event <- EV_END }()

	for !c.t.isquit() {
		pkt = get_packet()

		n, err = c.conn.Read(pkt.buf[:])
		if err != nil {
			if !strings.HasSuffix(err.Error(), "use of closed network connection") {
				logcli.Err(err)
			}
			put_packet(pkt)
			continue
		}

		err = pkt.Unpack(n)
		if err != nil {
			put_packet(pkt)
			logcli.Err(err)
			continue
		}
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
		delete(connlog, localaddr.String())
		fmt.Println(connlog)
	}

	c := &Client{t, conn, name}
	go c.sender()
	go c.recver()

	t.c_event <- EV_CONNECT
	<- t.c_connect
	logcli.Info("create tunnel", localaddr)
	connlog[localaddr.String()] = t
	return NewTunnelConn(t), nil
}
