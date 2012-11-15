package tunnel

import (
	"errors"
	"fmt"
	"net"
	"../sutils"
)

var logsrv *sutils.Logger

func init () {
	logsrv = sutils.NewLogger("server")
}

type Server struct {
	conn *net.UDPConn
	dispatcher map[string]*Tunnel
	handler func (net.Conn) (error)
	c_send chan *DataBlock
}

func NewServer(conn *net.UDPConn) (srv *Server) {
	srv = new(Server)
	srv.conn = conn
	srv.dispatcher = make(map[string]*Tunnel)
	srv.c_send = make(chan *DataBlock, 1)
	go srv.sender()
	return
}

func (srv *Server) sender () {
	var err error
	var db *DataBlock
	for {
		db, _ = <- srv.c_send
		_, err = srv.conn.WriteToUDP(db.buf, db.remote)
		if err != nil { logsrv.Err(err) }
	}
}

func (srv *Server) get_tunnel(remote *net.UDPAddr, buf []byte) (t *Tunnel, err error) {
	var ok bool
	remotekey := remote.String()
	t, ok = srv.dispatcher[remotekey]
	if ok {
		// logger.Debug("[server] dispatch to " + t.Dump())
		return
	}

	if buf[0] != SYN {
		return nil, errors.New("packet to unknow tunnel, " + remotekey)
	}

	t = NewTunnel(remote, fmt.Sprintf("%d_srv", remote.Port))
	t.c_send = srv.c_send
	t.onclose = func () {
		logsrv.Info("close tunnel", remotekey)
		delete(srv.dispatcher, remotekey)
	}

	srv.dispatcher[remotekey] = t
	go srv.handler(NewTunnelConn(t))
	logsrv.Info("create tunnel", remotekey)
	return
}	

func UdpServer (addr string, handler func (net.Conn) (error)) (err error) {
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil { return }
	defer conn.Close()

	srv := NewServer(conn)
	srv.handler = handler

	var n int
	var buf []byte
	var remote *net.UDPAddr
	var t *Tunnel

	for {
		buf = make([]byte, 2048)
		n, remote, err = conn.ReadFromUDP(buf)
		if err != nil {
			logsrv.Err(err)
			continue
		}

		t, err = srv.get_tunnel(remote, buf[:n])
		if err != nil {
			logsrv.Err(err)
			continue
		}
		if t == nil {
			logsrv.Err("unknow problem leadto channel 0")
			continue
		}

		t.c_recv <- buf[:n]
	}
	return
}
