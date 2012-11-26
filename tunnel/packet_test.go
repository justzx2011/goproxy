package tunnel

import (
	"bytes"
	"os"
	"testing"
)

var data []byte

func rand_data () (data []byte, err error) {
	data = make([]byte, MSS)

	f, err := os.Open("/dev/urandom")
	if err != nil { return }
	defer f.Close()

	_, err = f.Read(data)
	if err != nil { return }

	return
}

func copypkt (t *testing.T, pkt *Packet) (p *Packet) {
	err := pkt.Pack()
	if err != nil { t.Errorf(err.Error()) }

	p = get_packet()
	p.n = pkt.n
	i := copy(p.buf[:p.n], pkt.buf[:pkt.n])
	if i != pkt.n {
		t.Errorf("not copy for all bytes")
	}

	err = p.Unpack()
	if err != nil { t.Errorf(err.Error()) }
	return
}

func PackOnce (t *testing.T, flag uint8) {
	tick := get_nettick()

	pkt := get_packet()
	s := copy(pkt.buf[HEADERSIZE:], data)
	pkt.content = pkt.buf[HEADERSIZE:HEADERSIZE+s]
	if s != MSS { t.Errorf("half packet not full all data") }

	pkt.flag = flag
	pkt.window = 0
	pkt.seq = 10
	pkt.ack = 100
	pkt.sndtime = tick
	pkt.acktime = 0

	p := copypkt(t, pkt)
	put_packet(pkt)

	if p.flag != flag { t.Errorf("flag not match") }
	if p.window != 0 { t.Errorf("window not match") }
	if p.seq != 10 { t.Errorf("seq not match") }
	if p.ack != 100 { t.Errorf("ack not match") }
	if p.sndtime != tick { t.Errorf("send time not match") }
	if p.acktime != 0 { t.Errorf("ack time not match") }
	if !bytes.Equal(p.content, data) { t.Errorf("data not match") }
	put_packet(p)
}

func PackOnceFail (t *testing.T, flag uint8) {
	tick := get_nettick()
	data, err := rand_data()
	if err != nil { t.Errorf("rand data init failed") }

	pkt := get_packet()
	n := copy(pkt.buf[HEADERSIZE:], data)
	pkt.content = pkt.buf[HEADERSIZE:HEADERSIZE+n]
	if n != MSS { t.Errorf("half packet not full all data") }

	pkt.flag = flag
	pkt.window = 0
	pkt.seq = 10
	pkt.ack = 100
	pkt.sndtime = tick
	pkt.acktime = 0

	err = pkt.Pack()
	if err != nil { t.Errorf(err.Error()) }

	p := get_packet()
	p.n = pkt.n
	i := copy(p.buf[:p.n], pkt.buf[:pkt.n])
	if i != pkt.n { t.Errorf("not copy for all bytes") }
	p.buf[44] = p.buf[44] + 1

	err = p.Unpack()
	if err == nil {
		t.Errorf("crc wrong")
	}
	put_packet(pkt)
	put_packet(p)
}

func TestPack (t *testing.T) {
	var err error
	data, err = rand_data()
	if err != nil { t.Errorf("rand data init failed") }

	for i := 0; i < 1000; i++ {
		PackOnce(t, DAT)
		PackOnceFail(t, DAT)
		PackOnce(t, DAT | ACK)
		PackOnce(t, SYN)
		PackOnce(t, SYN | ACK)
		PackOnce(t, FIN)
		PackOnce(t, FIN | ACK)
	}
}
