package tunnel

import (
	"bytes"
	"net"
)

// create and close
// sync

type TunnelConn struct {
	conn net.Conn
	sendseq int32
	recvseq int32
	sendbuf []*Packet
	recvbuf []*Packet
	buf bytes.Buffer
	// sync.
}

func (tc TunnelConn) OnPacket(pkt *Packet) (err error) {
	switch(pkt.flag){
	case PKT_DATA:
		err = tc.OnData(pkt)
		if err != nil { return }
	case PKT_STATUS:
		// filter sendbuf for bi.seq < pkt.seq
		var sendbuf []*Packet
		for _, p := range tc.sendbuf {
			if p.seq >= pkt.seq {
				sendbuf = append(sendbuf, p)
			}
		}
		tc.sendbuf = sendbuf
	case PKT_GET:
		tc.sendStatus()
	case PKT_CLOSING:
	}
}

func (tc TunnelConn) OnData(pkt *Packet) (err error) {
	switch{
	case (pkt.seq - tc.recvseq) < 0:
		return
	case pkt.seq == tc.recvseq:
		tc.buf.Write(pkt.content)
		tc.recvseq += len(pkt.content)
		for tc.searchRecvBuf() { }
		tc.sendStatus()
		// TODO: send event
	case (pkt.seq - tc.recvseq) > 0:
		tc.recvbuf = append(tc.recvbuf, pkt)
	}
}

func (tc TunnelConn) searchRecvBuf() bool {
	for i, p = range tc.recvbuf {
		if  p.seq != tc.recvseq { continue }
		tc.buf.Write(pkt.content)
		tc.recvseq += len(pkt.content)
		copy(tc.recvbuf[i:], tc.recvbuf[i+1:])
		tc.recvbuf = tc.recvbuf[:len(tc.recvbuf)-1]
		return true
	}
	return false
}

func (tc TunnelConn) sendStatus() (err error) {
	return tc.send(&Packet{PKT_STATUS, tc.recvseq, []byte{}})
}

func (tc TunnelConn) send(pkt *Packet) (err error) {
	buf, err := pkt.Pack()
	if err != nil { return }

	n, err = tc.conn.Write(buf)
	if n != len(buf) {
		err = errors.New("send buffer full")
	}
	if err != nil { return }

	tc.sendbuf = append(tc.sendbuf, pkt)
	return
}

func (tc TunnelConn) Read(b []byte) (n int, err error) {
	for {
		n, err = tc.buf.Read(b)
		switch err {
		default: return
		case nil: return
		case io.EOF:
			// TODO: wait when eof
		}
	}
	return
}

func (tc TunnelConn) Write(b []byte) (n int, err error) {
	var pkt *Packet

	n = len(b)
	err = SplitBytes(b, PACKETSIZE, func (bi []byte) (err error){

		// TODO: 拥塞控制算法

		pkt = &Packet{PKT_DATA, tc.sendseq, bi}
		tc.sendseq += len(bi)

		err = tc.send(pkt)
		return 
	})
	return
}

func (tc TunnelConn) Close() error {
	pkt = &Packet{PKT_CLOSE, tc.sendseq, []byte{}}
	
}

func (tc TunnelConn) LocalAddr() net.Addr {
	return
}

func (tc TunnelConn) RemoteAddr() net.Addr {
	return
}

func (tc TunnelConn) SetDeadline(t time.Time) error {
	return
}

func (tc TunnelConn) SetReadDeadline(t time.Time) error {
	return
}

func (tc TunnelConn) SetWriteDeadline(t time.Time) error {
	return
}