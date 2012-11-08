package tunnel

type Tunnel struct {
	conn *net.Conn
	dispatcher map[int]Pair
	c chan Packet
}

func (t *Tunnel) main () {
	var n int
	var readbuf []byte

	for {
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

type Pair struct {
	t *Tunnel
	id uint16
	buf []byte
	rc chan Packet
}

func (p *Pair) Read(b []byte) (n int, err error) {
	for len(p.buf) == 0 {
		pkt := <- p.rc
		append(buf, pkt.content...)
	}
	n = copy(b, p.buf)
	p.buf = p.buf[n:]
	b = b[:n]
	return
}

func (p *Pair) Write(b []byte) (n int, err error) {
	if len(b) > 1024 {
		err = errors.New("packet too long")
		return
	}
	pkt := Packet{p.id, b}
	p.t.c <- pkt
	n = len(b)
	return 
}

type Packet struct {
	id uint16
	content []byte
}

func PacketUnpack(buf []byte) (p *Packet) {
	var p Packet
	reader := bytes.Buffer(buf)
	binary.Read(reader, binary.BigEndian, &(p.id))
	binary.Read(reader, binary.BigEndian, &(p.flag))
	p.content = reader.Bytes()
	return &p
}

func (p *Packet) Pack() (buf []byte, err error) {
	writer := bytes.Buffer(buf)
	binary.Write(writer, binary.BigEndian, &(m.id))
	binary.Write(writer, binary.BigEndian, &(m.flag))
	writer.Write(m.content)
	if len(buf) > 1024 {
		err = errors.New("packet too long")
	}
	return
}