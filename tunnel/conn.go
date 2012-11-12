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
	var ok bool
	b, ok = <- tc.t.c_read
	n = len(b)
	if !ok { err = io.EOF }
	log.Println(b)
	return
}

func (tc TunnelConn) Write(b []byte) (n int, err error) {
	n = len(b)
	err = SplitBytes(b, PACKETSIZE, func (bi []byte) (err error){
		tc.t.c_write <- bi
		return 
	})
	return
}

func (tc TunnelConn) Close() (err error) {
	tc.t.c_closing <- 1
	<- tc.t.c_closed
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
	// return tc.t.conn.SetDeadline(t)
	return nil
}

func (tc TunnelConn) SetReadDeadline(t time.Time) error {
	// return tc.t.conn.SetReadDeadline(t)
	return nil
}

func (tc TunnelConn) SetWriteDeadline(t time.Time) error {
	// return tc.t.conn.SetWriteDeadline(t)
	return nil
}
