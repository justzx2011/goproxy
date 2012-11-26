package tunnel

import (
	"io"
	"net"
	"time"
)

type TunnelConn struct {
	t *Tunnel
	local net.Addr
}

func (tc TunnelConn) Read(b []byte) (n int, err error) {
	var ok bool
	var l int

	defer func () {
		if x := recover(); x != nil { err = io.EOF }
	}()

	for {
		tc.t.readlck.Lock()
		l := tc.t.readbuf.Len()
		tc.t.readlck.Unlock()

		if l > 0 { break }
		_, ok = <- tc.t.c_read
		if !ok { return 0, io.EOF }
	}

	tc.t.readlck.Lock()
	n, err = tc.t.readbuf.Read(b)
	tc.t.readlck.Unlock()
	if err != nil { panic(err) }

	if l >= RESTARTACK && (l - n) < RESTARTACK {
		tc.t.c_event <- EV_READ
	}

	if l > n && tc.t.status == EST {
		select {
		case tc.t.c_read <- 1:
		default:
		}
	}
	return
}

func (tc TunnelConn) Write(b []byte) (n int, err error) {
	var size int
	var pkt *Packet

	defer func () {
		if x := recover(); x != nil { err = io.EOF }
	}()
	
	n = 0
	for i := 0; i < len(b); i += MSS {
		if len(b) - i >= MSS {
			size = MSS
		}else{ size = len(b) - i }

		pkt = get_packet()
		s := copy(pkt.buf[HEADERSIZE:HEADERSIZE+size], b[i:i+size])
		if s != size { panic("not all buffer copied to packet") }
		pkt.content = pkt.buf[HEADERSIZE:HEADERSIZE+size]

		if tc.t.status == CLOSED { return 0, io.EOF }
		tc.t.c_wrin <- pkt
		n += size
	}
	return
}

func (tc TunnelConn) Close() (err error) {
	if tc.t.isquit() { return }
	if tc.t.status == EST { tc.t.c_event <- EV_CLOSE }
	<- tc.t.c_close
	return
}

func (tc TunnelConn) LocalAddr() net.Addr {
	return tc.local
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
		// if err != nil { return }
		if err != nil { panic(err) }
	}
	return
}
