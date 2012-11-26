package tunnel

import (
	"fmt"
	"net"
	"sync"
	"../sutils"
)

var statsrv Statistics

type Server struct {
	conn *net.UDPConn
	addr *net.UDPAddr
	lock sync.Locker
	dispatcher map[string]*Tunnel
	handler func (net.Conn) (error)
	c_send chan *SendBlock
}

func NewServer(conn *net.UDPConn, addr *net.UDPAddr) (srv *Server) {
	srv = new(Server)
	srv.conn = conn
	srv.addr = addr
	srv.lock = new(sync.Mutex)
	srv.dispatcher = make(map[string]*Tunnel)
	srv.c_send = make(chan *SendBlock, TBUFSIZE)
	go srv.sender()
	return
}

func (srv *Server) sender () {
	var err error
	var n, ns int
	var db *SendBlock

	for {
		db = <- srv.c_send

		ns, err = srv.conn.WriteToUDP(db.pkt.buf[:db.pkt.n], db.remote)
		if err != nil {
			sutils.Err("WriteToUDP", err)
			statsrv.senderr += 1
		}
		if ns != db.pkt.n {
			sutils.Err("Write don't send all buffer")
		}
		statsrv.sendpkt += 1
		statsrv.sendsize += uint64(n)
	}
}

func (srv *Server) get_tunnel(remote *net.UDPAddr, pkt *Packet, local net.Addr) (t *Tunnel, err error) {
	var ok bool
	remotekey := remote.String()

	t, ok = srv.dispatcher[remotekey]
	if ok { return }

	if uint8(pkt.buf[0]) != SYN {
		sutils.Info("packet to unknow tunnel,", remotekey)
		p := get_packet()
		p.content = p.buf[HEADERSIZE:HEADERSIZE]
		p.flag = RST
		p.seq = 0
		p.ack = pkt.seq
		srv.c_send <- &SendBlock{remote, p}
		return nil, nil
	}

	t = NewTunnel(remote, fmt.Sprintf("%d_srv", remote.Port), srv.c_send)
	t.onclose = func () {
		srv.lock.Lock()
		defer srv.lock.Unlock()
		sutils.Info("close tunnel", remotekey)
		delete(srv.dispatcher, remotekey)
	}

	srv.lock.Lock()
	defer srv.lock.Unlock()
	srv.dispatcher[remotekey] = t
	go srv.handler(&TunnelConn{t, local})
	sutils.Info("create tunnel", remotekey)

	return
}	

func (srv *Server) recver () {
	var n int
	var err error
	var pkt *Packet
	var remote *net.UDPAddr
	var t *Tunnel

	for {
		pkt = get_packet()
		pkt.n, remote, err = srv.conn.ReadFromUDP(pkt.buf[:])
		if err != nil {
			statsrv.recverr += 1
			put_packet(pkt)
			sutils.Err("ReadFromUDP", err)
			continue
		}

		t, err = srv.get_tunnel(remote, pkt, srv.addr)
		if err != nil {
			statsrv.recverr += 1
			sutils.Err("get tunnel", err)
			continue
		}
		if t == nil {
			statsrv.recverr += 1
			continue
		}

		statsrv.recvpkt += 1
		statsrv.recvsize += uint64(n)
		t.c_recv <- pkt
	}
}

func UdpServer (addr string, handler func (net.Conn) (error)) (err error) {
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil { panic(err) }
	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil { panic(err) }
	defer conn.Close()

	srv := NewServer(conn, udpaddr)
	srv.handler = handler

	srv.recver()
	return
}
