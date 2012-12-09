package cryptconn

import (
	"crypto/cipher"
	"encoding/hex"
	"net"
	"../sutils"
)

type CryptConn struct {
	*net.TCPConn
	in cipher.Stream
	out cipher.Stream
}

func (sc CryptConn) Read(b []byte) (n int, err error) {
	n, err = sc.TCPConn.Read(b)
	if err != nil { return }
	sc.in.XORKeyStream(b[:n], b[:n])
	sutils.Debug("recv", hex.Dump(b[:n]))
	return 
}

func (sc CryptConn) Write(b []byte) (n int, err error) {
	sutils.Debug("send", hex.Dump(b))
	sc.out.XORKeyStream(b[:], b[:])
	return sc.TCPConn.Write(b)
}