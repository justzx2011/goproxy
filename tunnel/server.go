package tunnel

import (
	"errors"
	"fmt"
	"os"
	"net"
	"runtime/pprof"
	"../sutils"
)

var logsrv *sutils.Logger
var statsrv Statistics

func init () {
	logsrv = sutils.NewLogger("server")
}

type Server struct {
	conn *net.UDPConn
	dispatcher map[string]*Tunnel
	handler func (net.Conn) (error)
	c_send chan *SendBlock
}

func NewServer(conn *net.UDPConn) (srv *Server) {
	srv = new(Server)
	srv.conn = conn
	srv.dispatcher = make(map[string]*Tunnel)
	srv.c_send = make(chan *SendBlock, 1)
	go srv.sender()
	return
}

func (srv *Server) sender () {
	var err error
	var n int
	var db *SendBlock
	for {
		db = <- srv.c_send

		n, err = db.pkt.Pack()
		if err != nil {
			logsrv.Err("Pack", err)
			statsrv.senderr += 1
			continue
		}

		_, err = srv.conn.WriteToUDP(db.pkt.buf[:n], db.remote)
		if err != nil {
			logsrv.Err("WriteToUDP", err)
			statsrv.senderr += 1
		}
		statsrv.sendpkt += 1
		statsrv.sendsize += uint64(n)
	}
}

func (srv *Server) get_tunnel(remote *net.UDPAddr, pkt *Packet) (t *Tunnel, err error) {
	var ok bool
	remotekey := remote.String()
	t, ok = srv.dispatcher[remotekey]
	if ok { return }

	if pkt.flag != SYN {
		return nil, errors.New("packet to unknow tunnel, " + remotekey)
	}

	t = NewTunnel(remote, fmt.Sprintf("%d_srv", remote.Port))
	t.c_send = srv.c_send
	t.onclose = func () {
		logsrv.Info("close tunnel", remotekey)
		delete(srv.dispatcher, remotekey)
		logsrv.Debug(srv.dispatcher)
		pprof.StopCPUProfile()
	}

	srv.dispatcher[remotekey] = t
	go srv.handler(NewTunnelConn(t))
	logsrv.Info("create tunnel", remotekey)

        fo, err := os.Create("/tmp/cli.prof")
        if err != nil { panic(err) }
        pprof.StartCPUProfile(fo)

	return
}	

func UdpServer (addr string, handler func (net.Conn) (error)) (err error) {
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	// if err != nil { return }
	if err != nil { panic(err) }
	conn, err := net.ListenUDP("udp", udpaddr)
	// if err != nil { return }
	if err != nil { panic(err) }
	defer conn.Close()

	srv := NewServer(conn)
	srv.handler = handler

	var n int
	var pkt *Packet
	var remote *net.UDPAddr
	var t *Tunnel

	for {
		pkt = get_packet()
		n, remote, err = conn.ReadFromUDP(pkt.buf[:])
		if err != nil {
			statsrv.recverr += 1
			put_packet(pkt)
			logsrv.Err("ReadFromUDP", err)
			continue
		}

		err = pkt.Unpack(n)
		if err != nil {
			statsrv.recverr += 1
			logsrv.Err("Unpack", err)
			continue
		}

		t, err = srv.get_tunnel(remote, pkt)
		if err != nil {
			statsrv.recverr += 1
			logsrv.Err("get tunnel", err)
			continue
		}
		if t == nil {
			statsrv.recverr += 1
			logsrv.Err("unknow problem leadto channel 0")
			continue
		}

		statsrv.recvpkt += 1
		statsrv.recvsize += uint64(n)
		t.c_recv <- pkt

		logsrv.Debug("stat srv", statsrv)
	}
	return
}
