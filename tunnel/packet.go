package tunnel

import (
	"encoding/binary"
	"errors"
	"fmt"
	// "hash/crc32"
)

const HEADERSIZE = 25

var c_pktfree chan *Packet

func init () {
	c_pktfree = make(chan *Packet, 100)
}

func get_packet () (p *Packet) {
	select {
	case p = <- c_pktfree:
	default: p = new(Packet)
	}
	return
}

func put_packet (p *Packet) {
	select {
	case c_pktfree <- p:
	default:
	}
}

const (
	DAT = uint8(0x00)
	SYN = uint8(0x01)
	FIN = uint8(0x02)
	RST = uint8(0x03)
	PST = uint8(0x04)
	SACK = uint8(0x05)
	ACK = uint8(0x80)
	ACKMASK = ^(ACK)
)

func DumpFlag(flag uint8) (r string) {
	var rs string
	if flag == ACK { return "ACK" }
	switch flag & ACKMASK {
	case DAT: rs = "DAT"
	case SYN: rs = "SYN"
	case FIN: rs = "FIN"
	case RST: rs = "RST"
	case PST: rs = "PST"
	case SACK: rs = "SACK"
	}
	if (flag & ACK) != 0 { rs = rs + "|ACK" }
	return rs
}

type Packet struct {
	flag uint8
	window uint32
	seq int32
	ack int32
	sndtime int32
	acktime int32
	content []byte

	n int
	buf [MSS+HEADERSIZE]byte
}

func (p *Packet) read_status (t *Tunnel, flag uint8) {
	p.flag = flag
	t.readlck.Lock()
	l := t.readbuf.Len()
	t.readlck.Unlock()
	if WINDOWSIZE > l {
		p.window = uint32(WINDOWSIZE - l)
	}else{ p.window = 0 }
	p.seq = t.seq_send
	p.ack = t.seq_recv
}

func (p *Packet) String() string {
	return fmt.Sprintf("f: %s, seq: %d, ack: %d, wnd: %d, len: %d, stm: %d, atm: %d",
		DumpFlag(p.flag), p.seq, p.ack, p.window, len(p.content),
		p.sndtime, p.acktime)
}

func checksum(buf []byte) uint16 {
	var c [2]byte
	size := len(buf)
	if size % 2 == 1 {
		c[0] ^= buf[size-1]
		size -= 1
	}
	for i := 0; i < size; i+=2 {
		c[0] ^= buf[i]
		c[1] ^= buf[i+1]
	}
	return uint16(c[0])<<8 + uint16(c[1])
}

func (p *Packet) Pack() (err error) {
	size := len(p.content)
	if size > MSS {
		return fmt.Errorf("packet too large, %d/%d", len(p.content), MSS)
	}

	p.buf[0] = byte(p.flag)
	binary.BigEndian.PutUint32(p.buf[1:5], uint32(p.window))
	binary.BigEndian.PutUint32(p.buf[5:9], uint32(p.seq))
	binary.BigEndian.PutUint32(p.buf[9:13], uint32(p.ack))
	binary.BigEndian.PutUint32(p.buf[13:17], uint32(p.sndtime))
	binary.BigEndian.PutUint32(p.buf[17:21], uint32(p.acktime))
	binary.BigEndian.PutUint16(p.buf[21:23], uint16(size))

	crc := checksum(p.buf[:23])
	if size != 0 { crc ^= checksum(p.content) }
	binary.BigEndian.PutUint16(p.buf[23:25], crc)
	
	// crc := crc32.Update(0, crc32.IEEETable, p.buf[:23])
	// if size != 0 {
	// 	crc = crc32.Update(crc, crc32.IEEETable, p.content)
	// }
	// binary.BigEndian.PutUint16(p.buf[23:25], uint16(crc))

	p.n = HEADERSIZE+size
	return err
}

func (p *Packet) Unpack() (err error) {
	p.flag = uint8(p.buf[0])
	p.window = binary.BigEndian.Uint32(p.buf[1:5])
	p.seq = int32(binary.BigEndian.Uint32(p.buf[5:9]))
	p.ack = int32(binary.BigEndian.Uint32(p.buf[9:13]))
	p.sndtime = int32(binary.BigEndian.Uint32(p.buf[13:17]))
	p.acktime = int32(binary.BigEndian.Uint32(p.buf[17:21]))
	size := uint16(binary.BigEndian.Uint16(p.buf[21:23]))
	crc1 := uint16(binary.BigEndian.Uint16(p.buf[23:25]))

	if p.n != HEADERSIZE + int(size) { return errors.New("packet broken") }
	p.content = p.buf[HEADERSIZE:HEADERSIZE+size]

	crc := checksum(p.buf[:23])
	if size != 0 { crc ^= checksum(p.content) }

	// crc := crc32.Update(0, crc32.IEEETable, p.buf[:23])
	// if len(p.content) != 0 {
	// 	crc = crc32.Update(crc, crc32.IEEETable, p.content)
	// }

	if crc1 != uint16(crc) {
		return fmt.Errorf("crc32 fault %x/%x %s", crc1, uint16(crc), p)
	}
	return
}
