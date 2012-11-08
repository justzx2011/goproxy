package udptunnel

import (
	"bufio"
	"encoding/binary"
	"errors"
	// "io"
)

const (
	SYN = 1
	ACK = 2
	PSH = 3
	FIN = 4
	RST = 5
)

const PACKETSIZE = 512

type Packet struct {
	id uint16
	seq int32
	ack int32
	flag uint8
	// window uint32
	content []byte
}

func (m *Packet)ReadFrom(conn *io.Reader) (err error) {
	var n int
	var buf [2048]byte

	n, err = conn.Read(buf[:])
	if err != nil { return }

	reader := bytes.Buffer(buf)
	binary.Read(reader, binary.BigEndian, &(m.id))
	binary.Read(reader, binary.BigEndian, &(m.seq))
	binary.Read(reader, binary.BigEndian, &(m.ack))
	binary.Read(reader, binary.BigEndian, &(m.flag))
	m.content = reader.Bytes()

	return
}

func (m *Packet)WriteTo(conn *io.Writer) (err error) {
	var n int
	var buf [2048]byte

	writer := bytes.Buffer(buf)

	binary.Write(writer, binary.BigEndian, &(m.id))
	binary.Write(writer, binary.BigEndian, &(m.seq))
	binary.Write(writer, binary.BigEndian, &(m.ack))
	binary.Write(writer, binary.BigEndian, &(m.flag))
	writer.Write(m.content)

	conn.Write()
	err = writer.WriteByte(m.msgtype)
	if err != nil { return }
	binary.Write(writer, binary.BigEndian, len(m.content))

	n, err = writer.Write(m.content)
	if err != nil { return }
	if n != len(m.content) {
		err = errors.New("writer buffer full")
		return
	}

	writer.Flush()
	return
}