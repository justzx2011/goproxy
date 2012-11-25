package tunnel

import (
	"fmt"
	"net"
	"strings"
	"../sutils"
)

var logcli *sutils.Logger
var connlog map[string]*Tunnel
var statcli Statistics

func init () {
	logcli = sutils.NewLogger("client")
	connlog = make(map[string]*Tunnel)
}

type Client struct {
	t *Tunnel
	conn *net.UDPConn
	name string
	c_close chan uint8
}

func (c *Client) isquit () (bool) {
	select {
	case <- c.c_close: return true
	default:
	}
	return false
}

func (c *Client) sender () {
	var err error
	var n, ns int
	var db *SendBlock
	defer func () {
		c.t.logger.Debug("client sender quit")
		c.t.c_event <- EV_END
	}()

	for !c.isquit() {
		db = <- c.t.c_send
		if db == nil { break }

		n, err = db.pkt.Pack()
		if err != nil {
			logcli.Err("Pack", err)
			statcli.senderr += 1
			continue
		}

		ns, err = c.conn.Write(db.pkt.buf[:n])
		if err != nil {
			statcli.senderr += 1
			if strings.HasSuffix(err.Error(), "use of closed network connection") {
				break
			}
			logcli.Err("Write Net", err)
			continue
		}
		if ns != n {
			logcli.Err("Write don't send all buffer")
		}
		statcli.sendpkt += 1
		statcli.sendsize += uint64(n)
	}
}

func (c *Client) recver () {
	var err error
	var n int
	var pkt *Packet
	defer func () {
		c.t.logger.Debug("client recver quit")
		c.t.c_event <- EV_END
	}()

	for !c.isquit() {
		pkt = get_packet()

		n, err = c.conn.Read(pkt.buf[:])
		// fixme: remove this
		c.t.logger.Debug("something readed in client sender")
		if err != nil {
			statcli.recverr += 1
			if !strings.HasSuffix(err.Error(), "use of closed network connection") {
				logcli.Err("Read Net", err)
			}
			put_packet(pkt)
			continue
		}

		err = pkt.Unpack(n)
		if err != nil {
			statcli.recverr += 1
			put_packet(pkt)
			logcli.Err("Unpack", err)
			continue
		}
		statcli.recvpkt += 1
		statcli.recvsize += uint64(n)
		c.t.c_recv <- pkt

		// logcli.Debug("stat cli", statcli)
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
	localstr := localaddr.String()

	name := fmt.Sprintf("%s_cli", strings.Split(localstr, ":")[1])
	t = NewTunnel(udpaddr, name)
	c := &Client{t, conn, name, make(chan uint8)}

	t.c_send = make(chan *SendBlock, 1)
	t.onclose = func () {
		logcli.Info("close tunnel", localaddr)
		conn.Close()
		close(c.c_close)
		close(t.c_send)

		delete(connlog, localstr)
		logcli.Debug(connlog)
	}
	go c.sender()
	go c.recver()

	t.c_event <- EV_CONNECT
	<- t.c_connect
	logcli.Info("create tunnel", localaddr)
	connlog[localstr] = t

	return &TunnelConn{t, localaddr}, nil
}
