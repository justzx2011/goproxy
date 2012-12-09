package tunnel

import (
	"io"
)

var c_buffree chan []byte

func init () {
	c_buffree = make(chan []byte, 10)
}

func get_buf() (b []byte) {
	select {
	case b = <- c_buffree:
	default: b = make([]byte, 2*MSS)
	}
	return
}

func put_buf (b []byte) {
	select {
	case c_buffree <- b:
	default:
	}
}

func coreCopy(dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}

func Copy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := get_buf()
	defer put_buf(buf)

	return coreCopy(dst, src, buf)
}

func CopySize(dst io.Writer, src io.Reader, size int) (written int64, err error) {
	buf := make([]byte, size)
	return coreCopy(dst, src, buf)
}