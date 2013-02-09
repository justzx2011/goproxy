package cryptconn

import (
	"crypto/cipher"
	"encoding/hex"
	"io"
	"net"
	"../sutils"
)

const DEBUGOUTPUT bool = true

type CryptConn struct {
	*net.TCPConn
	in cipher.Stream
	out cipher.Stream
}

func (sc CryptConn) Read(b []byte) (n int, err error) {
	n, err = sc.TCPConn.Read(b)
	if err != nil { return }
	sc.in.XORKeyStream(b[:n], b[:n])
	if DEBUGOUTPUT {
		sutils.Debug("recv\n", hex.Dump(b[:n]))
	}
	return 
}

type writerOnly struct {
	io.Writer
}

func (sc CryptConn) ReadFrom(r io.Reader) (n int64, err error) {
	sutils.Debug("cryptconn readfrom call")
	return io.Copy(writerOnly{sc}, r)
}

func (sc CryptConn) Write(b []byte) (n int, err error) {
	if DEBUGOUTPUT {
		sutils.Debug("send\n", hex.Dump(b))
	}
	sc.out.XORKeyStream(b[:], b[:])
	return sc.TCPConn.Write(b)
}