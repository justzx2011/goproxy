package tunnel

type Pair struct {
	t *Tunnel
	clientid uint16
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
