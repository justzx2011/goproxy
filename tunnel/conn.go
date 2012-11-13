package tunnel

import (
	"bytes"
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
		_, err = tc.buf.Write(<- tc.t.c_read)
		if err != nil { return }
	}
	return tc.buf.Read(b)
}

func (tc TunnelConn) Write(b []byte) (n int, err error) {
	err = SplitBytes(b, PACKETSIZE, func (bi []byte) (err error){
		tc.t.c_write <- bi
		return 
	})
	n = len(b)
	return
}

func (tc TunnelConn) Close() (err error) {
	tc.t.c_evin <- FIN
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
