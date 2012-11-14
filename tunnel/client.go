package tunnel

import (
	"log"
	"math/rand"
	"net"
)

type Client struct {
	t *Tunnel
	conn *net.UDPConn
}

func (c *Client) sender () {
	var err error
	var ok bool
	var db *DataBlock

	for {
		db, ok = <- c.t.c_send
		if !ok { break }
		if DROPFLAG && rand.Intn(100) >= 85 {
			log.Println("DEBUG: client drop packet")
			continue
		}
		_, err = c.conn.Write(db.buf)
		if _, ok := err.(*net.OpError); ok {
			break
		}
		if err != nil {
			log.Println(err.Error())
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
			log.Println(err.Error())
			break
		}

		if DEBUG { log.Println("client recv", buf[:n]) }
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

	t = NewTunnel(udpaddr)
	t.c_send = make(chan *DataBlock, 1)
	t.onclose = func () {
		if WARNING { log.Println("close tunnel", localaddr, addr) }
		conn.Close()
	}

	c := &Client{t, conn}
	go c.sender()
	go c.recver()

	t.c_evin <- EV_CONNECT
	<- t.c_evout
	if WARNING { log.Println("create tunnel", localaddr, addr) }
	return NewTunnelConn(t), nil
}
