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
	var ok bool

	if tc.t.status == CLOSED {
		tc.t.logger.Debug("Read status EOF")
		return 0, io.EOF
	}

	tc.t.readlck.Lock()
	l := tc.t.readbuf.Len()
	tc.t.readlck.Unlock()
	if l == 0 {
		_, ok = <- tc.t.c_read
		if !ok { return 0, io.EOF }
	}

	tc.t.readlck.Lock()
	n, err = tc.t.readbuf.Read(b)
	tc.t.readlck.Unlock()
	if err != nil { return }

	if l > n && tc.t.status == EST {
		select {
		case tc.t.c_read <- 1:
		default:
		}
	}
	return
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

func SplitBytes(b []byte, size int, f func ([]byte) (error)) (err error) {
	for i := 0; i < len(b); i += size {
		if i + size < len(b) {
			err = f(b[i:i+size])
		}else{ err = f(b[i:]) }
		if err != nil { return }
	}
	return
}
