package tunnel

import (
	"bytes"
	"io"
	"net"
	"time"
)

type TunnelConn struct {
	t *Tunnel
	buf *bytes.Buffer
}

func NewTunnelConn(t *Tunnel) (tc *TunnelConn) {
	tc = new(TunnelConn)
	tc.t = t
	tc.buf = bytes.NewBuffer([]byte{})
	return
}

func (tc TunnelConn) Read(b []byte) (n int, err error) {
	if tc.buf.Len() == 0 {
		bi, ok := <- tc.t.c_read
		if !ok { return 0, io.EOF }
		_, err = tc.buf.Write(bi)
		if err != nil { return }
	}
	return tc.buf.Read(b)
}

func (tc TunnelConn) Write(b []byte) (n int, err error) {
	n = 0
	err = SplitBytes(b, PACKETSIZE, func (bi []byte) (err error){
		if tc.t.status == CLOSED { return io.EOF }
		tc.t.c_write <- bi
		n += len(bi)
		return 
	})
	return
}

func (tc TunnelConn) Close() (err error) {
	tc.t.c_evin <- EV_CLOSE
	<- tc.t.c_evout
	return
}

func (tc TunnelConn) LocalAddr() net.Addr {
	// return tc.t.conn.LocalAddr()
	// 哈哈
	return tc.t.remote
}

func (tc TunnelConn) RemoteAddr() net.Addr {
	return tc.t.remote
}

func (tc TunnelConn) SetDeadline(t time.Time) error {
	return nil
}

func (tc TunnelConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (tc TunnelConn) SetWriteDeadline(t time.Time) error {
	return nil
}
