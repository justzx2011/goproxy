package tunnel

type Server struct {
	conn net.Conn
	dispatcher map[net.UDPAddr]*TunnelConn
}

func UdpServer(addr string, handler func (net.Conn) (error)) (err error) {
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil { return }
	defer conn.Close()

	srv := &Server{conn, make(map[net.UDPAddr]*TunnelConn)}

	var n int
	var buf []byte
	var ok bool
	var pkt Packet
	var remote *net.UDPAddr
	var tc *TunnelConn

	for {
		buf = make([]byte, PACKETSIZE * 2)
		n, remote, err = conn.ReadFromUDP(buf)
		if err != nil { return }

		tc, ok = srv.dispatcher[*remote]
		if !ok {
			tc = &TunnelConn{conn}
			srv.dispatcher[*remote] = tc
			go handler(*tc)
		}

		pkt, err = Unpack(buf[:n])
		if err != nil {
			log.Println(err.Error())
			continue
		}

		err = tc.OnPacket(&pkt)
		if err != nil {
			log.Println(err.Error())
			continue
		}
	}
	return
}

func DialTunnel(addr string) (conn net.Conn, err error) {
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	udp, err = net.DialUDP("udp", nil, udpaddr)
	if err != nil { return }

	tc = &TunnelConn{udp}
	go client_main(tc)
	return *tc, nil
}

func client_main (tc *TunnelConn) {
	var err error
	var n int
	var buf []byte
	var pkt Packet

	for {
		buf = make([]byte, PACKETSIZE * 2)
		n, err = conn.Read(buf)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		pkt, err = Unpack(buf[:n])
		if err != nil {
			log.Println(err.Error())
			continue
		}
		err = tc.OnPacket(&pkt)
		if err != nil {
			log.Println(err.Error())
			continue
		}
	}
}