package tunnel

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
)

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
		if DROPFLAG && rand.Intn(100) >= 85 {
			logger.Debug(fmt.Sprintf("[%s] drop packet", c.name))
			continue
		}
		_, err = c.conn.Write(db.buf)
		if _, ok := err.(*net.OpError); ok {
			break
		}
		if err != nil {
			logger.Err("[client] " + err.Error())
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
			logger.Err(fmt.Sprintf("[client] %s", err.Error()))
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
		logger.Info(fmt.Sprintf("[client] close tunnel %s", localaddr))
		conn.Close()
	}

	c := &Client{t, conn, name}
	go c.sender()
	go c.recver()

	t.c_evin <- EV_CONNECT
	<- t.c_evout
	logger.Info(fmt.Sprintf("[client] create tunnel %s", localaddr))
	return NewTunnelConn(t), nil
}
