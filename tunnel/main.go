package tunnel

import (
	"log"
	"net"
)

type Server struct {
	conn net.Conn
	dispatcher map[string]*Tunnel
	// FIXME: 蛋疼
}

func UdpServer (addr string, handler func (net.Conn) (error)) (err error) {
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil { return }
	defer conn.Close()

	srv := &Server{conn, make(map[string]*Tunnel)}

	var n int
	var buf []byte
	var ok bool
	var remote *net.UDPAddr
	var t *Tunnel

	for {
		buf = make([]byte, PACKETSIZE * 2)
		n, remote, err = conn.ReadFromUDP(buf)
		if err != nil { return }

		log.Println("server", remote, buf[:n])

		t, ok = srv.dispatcher[remote.String()]
		log.Println("dispatch", t, ok)
		if !ok {
			if buf[0] != SYN {
				log.Println("packet to unknow tunnel")
				continue
			}
			t, err = NewTunnel(conn, remote)
			if err != nil { continue }
			srv.dispatcher[remote.String()] = t
			func (remote *net.UDPAddr) {
				t.onclose = func () {
					log.Println("close tunnel", remote.String())
					delete(srv.dispatcher, remote.String())
				}
			}(remote)
			go handler(TunnelConn{t})
			log.Println("create tunnel", t)
		}

		err = t.OnData(buf[:n])
		if err != nil {
			log.Println(err.Error())
			continue
		}
	}
	return
}

func DialTunnel(addr string) (conn net.Conn, err error) {
	var udpaddr *net.UDPAddr
	var udp *net.UDPConn
	var t *Tunnel

	udpaddr, err = net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	udp, err = net.DialUDP("udp", nil, udpaddr)
	if err != nil { return }

	t, err = NewTunnel(udp, nil)
	if err != nil { return }
	go client_main(t)
	t.status = SYNSENT
	err = t.send(SYN, []byte{})
	if err != nil { return }
	<- t.c_connect

	return TunnelConn{t}, nil
}

func client_main (t *Tunnel) {
	var err error
	var n int
	var buf []byte

	for {
		buf = make([]byte, PACKETSIZE * 2)
		n, err = t.conn.Read(buf)
		if err != nil {
			log.Println(err.Error())
			continue
		}

		log.Println("client", buf[:n])

		err = t.OnData(buf[:n])
		if err != nil {
			log.Println(err.Error())
			continue
		}
	}
}