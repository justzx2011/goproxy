package tunnel

import (
	"errors"
	"log"
	"net"
)

const DEBUG = false

type Server struct {
	conn *net.UDPConn
	dispatcher map[string]*Tunnel
	// FIXME: 蛋疼
	c_send chan *DataBlock
}

func UdpServer (addr string, handler func (net.Conn) (error)) (err error) {
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil { return }
	defer conn.Close()

	srv := &Server{conn, make(map[string]*Tunnel), make(chan *DataBlock, 10)}
	go srv.sender()

	var n int
	var buf []byte
	var remote *net.UDPAddr
	var t *Tunnel

	for {
		buf = make([]byte, PACKETSIZE * 2)
		n, remote, err = conn.ReadFromUDP(buf)
		if err != nil {
			log.Println(err.Error())
			continue
		}

		if DEBUG { log.Println("server recv", remote, buf[:n]) }

		t, err = srv.create_tunnel(remote, buf[:n], handler)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		if t == nil {
			log.Println("unknow problem leadto channel 0")
			continue
		}

		t.c_recv <- buf[:n]

		// err = t.OnData(buf[:n])
		// if err != nil {
		// 	log.Println(err.Error())
		// 	continue
		// }
	}
	return
}

func (srv *Server) create_tunnel(remote *net.UDPAddr, buf []byte, handler func (net.Conn) (error)) (t *Tunnel, err error) {
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
		log.Println("close tunnel", remotekey)
		delete(srv.dispatcher, remotekey)
	}

	srv.dispatcher[remotekey] = t
	go handler(TunnelConn{t})
	log.Println("create tunnel", remotekey)

	return
}	

func (srv *Server) sender () {
	var err error
	var db *DataBlock
	for {
		db = <- srv.c_send
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
	t.c_send = make(chan *DataBlock, 10)
	go client_sender(t, conn)
	go client_main(t, conn)

	t.status = SYNSENT
	err = t.send(SYN, []byte{})
	if err != nil { return }

	<- t.c_connect
	return TunnelConn{t}, nil
}

func client_sender (t *Tunnel, conn net.Conn) {
	var err error
	var db *DataBlock
	for {
		db = <- t.c_send
		_, err = conn.Write(db.buf)
		if err != nil { log.Println(err.Error()) }
	}
}

func client_main (t *Tunnel, conn net.Conn) {
	var err error
	var n int
	var buf []byte

	for {
		buf = make([]byte, PACKETSIZE * 2)
		n, err = conn.Read(buf)
		if err != nil {
			log.Println(err.Error())
			continue
		}

		if DEBUG { log.Println("client recv", buf[:n]) }
		t.c_recv <- buf[:n]
	}
}

type DataBlock struct {
	remote *net.UDPAddr
	buf []byte
}

