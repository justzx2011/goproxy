package tunnel

import (
	// "bytes"
	// "encoding/binary"
	"errors"
	"log"
	"net"
)

type Server struct {
	conn *net.UDPConn
	dispatcher map[string]*Tunnel
	c_send chan *DataBlock
}

func UdpServer (addr string, handler func (net.Conn) (error)) (err error) {
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil { return }
	defer conn.Close()

	srv := &Server{conn, make(map[string]*Tunnel), make(chan *DataBlock, 1)}
	go srv.sender()

	var n int
	var buf []byte
	var remote *net.UDPAddr
	var t *Tunnel

	for {
		buf = make([]byte, 2048)
		n, remote, err = conn.ReadFromUDP(buf)
		if err != nil {
			log.Println(err.Error())
			continue
		}

		if DEBUG { log.Println("server recv", remote, buf[:n]) }

		t, err = srv.get_tunnel(remote, buf[:n], handler)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		if t == nil {
			log.Println("unknow problem leadto channel 0")
			continue
		}

		t.c_recv <- buf[:n]
	}
	return
}

func (srv *Server) get_tunnel(remote *net.UDPAddr, buf []byte, handler func (net.Conn) (error)) (t *Tunnel, err error) {
	var ok bool
	remotekey := remote.String()
	t, ok = srv.dispatcher[remotekey]
	if DEBUG { log.Println("dispatch", t, ok) }
	if ok { return }

	if buf[0] != SYN {
		return nil, errors.New("packet to unknow tunnel, " + remotekey)
	}

	t, err = NewTunnel(remote)
	if err != nil { return }
	t.c_send = srv.c_send
	t.onclose = func () {
		if DEBUG { log.Println("close tunnel", remotekey) }
		delete(srv.dispatcher, remotekey)
	}

	srv.dispatcher[remotekey] = t
	go handler(NewTunnelConn(t))
	if DEBUG { log.Println("create tunnel", remotekey) }
	return
}	

func (srv *Server) sender () {
	var ok bool
	var err error
	var db *DataBlock
	for {
		db, ok = <- srv.c_send
		if !ok { return }
		_, err = srv.conn.WriteToUDP(db.buf, db.remote)
		if err != nil { log.Println(err.Error()) }
	}
}

func DialTunnel(addr string) (tc net.Conn, err error) {
	var udpaddr *net.UDPAddr
	var conn *net.UDPConn
	var t *Tunnel

	udpaddr, err = net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	conn, err = net.DialUDP("udp", nil, udpaddr)
	if err != nil { return }

	t, err = NewTunnel(nil)
	if err != nil { return }
	t.c_send = make(chan *DataBlock, 1)
	go client_sender(t, conn)
	go client_main(t, conn)

	t.c_evin <- SYN
	<- t.c_evout
	return NewTunnelConn(t), nil
}

func client_sender (t *Tunnel, conn net.Conn) {
	var err error
	var ok bool
	var db *DataBlock
	defer conn.Close()

	for {
		db, ok = <- t.c_send
		if !ok { break }
		_, err = conn.Write(db.buf)
		if err != nil { log.Println(err.Error()) }
	}
}

func client_main (t *Tunnel, conn net.Conn) {
	var err error
	var n int
	var buf []byte

	for {
		buf = make([]byte, 2048)
		n, err = conn.Read(buf)
		if err != nil {
			log.Println(err.Error())
			continue
		}

		if DEBUG { log.Println("client recv", buf[:n]) }
		t.c_recv <- buf[:n]
	}
}