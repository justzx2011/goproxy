package tunnel

import (
	"bytes"
	"io"
	"net"
	"sync/atomic"
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
		select {
		case <- tc.t.c_close:
			tc.t.logger.Debug("read event EOF")
			return 0, io.EOF
		case bi, ok := <- tc.t.c_read:
			if !ok {
				tc.t.logger.Debug("read EOF")
				return 0, io.EOF
			}
			_, err = tc.buf.Write(bi)
			if err != nil { return }
			read_event := tc.t.readlen > RESTARTACK
			atomic.AddInt32(&tc.t.readlen, -int32(len(bi)))
			read_event = read_event && (tc.t.readlen < RESTARTACK)
			if read_event { tc.t.c_event <- EV_READ }
		}
	}
	return tc.buf.Read(b)
}

func (tc TunnelConn) Write(b []byte) (n int, err error) {
	n = 0
	err = SplitBytes(b, SMSS, func (bi []byte) (err error){
		if tc.t.status == CLOSED {
			tc.t.logger.Debug("write status EOF")
			return io.EOF
		}
		select {
		case <- tc.t.c_close:
			tc.t.logger.Debug("write event EOF")
			return io.EOF
		case tc.t.c_write <- bi: n += len(bi)
		}
		return 
	})
	return
}

func (tc TunnelConn) Close() (err error) {
	if tc.t.status == CLOSED { return }
	tc.t.logger.Debug("closing")
	if tc.t.status == EST { tc.t.c_event <- EV_CLOSE }
	<- tc.t.c_close
	tc.t.logger.Debug("closed")
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
