package tunnel

type Packet struct {
	serverid uint16
	clientid uint16
	content []byte
}

func PacketUnpack(buf []byte) (p *Packet) {
	var p Packet
	reader := bytes.Buffer(buf)
	binary.Read(reader, binary.BigEndian, &(p.id))
	binary.Read(reader, binary.BigEndian, &(p.flag))
	p.content = reader.Bytes()
	return &p
}

func (p *Packet) Pack() (buf []byte, err error) {
	writer := bytes.Buffer(buf)
	binary.Write(writer, binary.BigEndian, &(m.id))
	binary.Write(writer, binary.BigEndian, &(m.flag))
	writer.Write(m.content)
	if len(buf) > 1024 {
		err = errors.New("packet too long")
	}
	return
}