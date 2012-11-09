package tunnel

type Tunnel struct {
	conn *net.Conn
	serverid uint32
	dispatcher map[int]Pair
	c chan Packet
}

func NewTunnel(addr net.UDPAddr) (t *Tunnel, err error) {
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil { return }

	t = &Tunnel{conn}
	// get serverid
	t.dispatcher = make(map[int]Pair)
	t.c = make(chan Packet)
	return
}

func (t *Tunnel) Send (b []byte) (err error) {
	
}

func (t *Tunnel) main () {
	var n int
	var readbuf []byte

	for {
		readbuf = make([]byte, 2048)
		select {
		case t.conn.Read(readbuf): // FIXME: length not enough
			pkt := PacketUnpack(readbuf)
			if pkt.id == 0 {
				// TODO: connecting, connected, closing, closed
			}
			p := dispatcher[pkt.id]
			p.rc <- pkt
		case pkt := <- t.c:
			buf := pkt.Pack()
			conn.Write(buf) // FIXME: buffer full
		}
	}
}