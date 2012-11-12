package tunnel

import (
	"io"
	"log"
	"net"
	"time"
)

type TunnelConn struct {
	t *Tunnel
}

func (tc TunnelConn) Read(b []byte) (n int, err error) {
	for {
		if tc.t.buf.Len() == 0 { <- tc.t.recvcha }
		n, err = tc.t.buf.Read(b)
		if DEBUG { log.Println("read", n, err) }
		if err == nil && tc.t.buf.Len() > 0 && len(tc.t.recvcha) == 0{
			tc.t.recvcha <- 1
		}
		if err != io.EOF { return }
	}
	return
}

func (tc TunnelConn) Write(b []byte) (n int, err error) {
	n = len(b)
	err = SplitBytes(b, PACKETSIZE, func (bi []byte) (err error){
		err = tc.t.send(0, bi)
		if err != nil { return }
		return 
	})
	// fixme: I should wait until sendbuf empty
	return
}

func (tc TunnelConn) Close() (err error) {
	tc.t.status = FINWAIT
	err = tc.t.send(FIN, []byte{})
	<- tc.t.c_close
	return
}

func (tc TunnelConn) LocalAddr() net.Addr {
	return tc.t.conn.LocalAddr()
}

func (tc TunnelConn) RemoteAddr() net.Addr {
	return tc.t.conn.RemoteAddr()
}

func (tc TunnelConn) SetDeadline(t time.Time) error {
	return tc.t.conn.SetDeadline(t)
}

func (tc TunnelConn) SetReadDeadline(t time.Time) error {
	return tc.t.conn.SetReadDeadline(t)
}

func (tc TunnelConn) SetWriteDeadline(t time.Time) error {
	return tc.t.conn.SetWriteDeadline(t)
}
