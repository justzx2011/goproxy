package sutils

import (
	"io"
)

var freelist = make(chan []byte, 64)

func GetBuf() (b []byte) {
        select {
        case b = <-freelist:
        default:
		b = make([]byte, 1024)
        }
	return
}

func FreeBuf(b []byte) {
	select {
	case freelist <- b:
	default:
	}
	return
}

func CoreCopy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := GetBuf()
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 { written += int64(nw) }
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF { break }
		if er != nil {
			err = er
			break
		}
	}
	FreeBuf(buf)
	return written, err
}

func CopyLink(src, dst io.ReadWriteCloser) {
	defer src.Close()
	defer dst.Close()
	go func () {
		defer src.Close()
		defer dst.Close()
		CoreCopy(src, dst)
	}()
	CoreCopy(dst, src)
}
