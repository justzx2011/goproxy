package secconn

import (
	"crypto/cipher"
	// "encoding/hex"
	"net"
	// "../sutils"
)

type SecConn struct {
	*net.TCPConn
	in cipher.Stream
	out cipher.Stream
}

func (sc SecConn) Read(b []byte) (n int, err error) {
	n, err = sc.TCPConn.Read(b)
	if err != nil { return }
	sc.in.XORKeyStream(b[:n], b[:n])
	// sutils.Debug("recv", hex.Dump(b[:n]))
	return 
}

func (sc SecConn) Write(b []byte) (n int, err error) {
	// sutils.Debug("send", hex.Dump(b))
	sc.out.XORKeyStream(b[:], b[:])
	n, err =  sc.TCPConn.Write(b)
	return
}