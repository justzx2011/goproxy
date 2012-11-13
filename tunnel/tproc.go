package tunnel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"time"
)

func (t *Tunnel) proc_packet(pkt *Packet) (err error) {
	if (pkt.flag & ACK) != 0 {
		switch t.status {
		case SYNRCVD:
			t.status = EST
		case LASTACK:
			t.status = CLOSED
			t.Close()
		}
	}

	if (pkt.flag & SYN) != 0 { return t.proc_syn(pkt) }
	if (pkt.flag & FIN) != 0 { return t.proc_fin(pkt) }
	if pkt.flag == SACK { return t.proc_sack(pkt) }

	if len(pkt.content) > 0 {
		t.recvseq += int32(len(pkt.content))
		t.c_read <- pkt.content
	}else if pkt.flag != ACK {
		t.recvseq += 1
	}

	return
}

func (t *Tunnel) proc_syn (pkt *Packet) (err error) {
	t.recvseq += 1
	if (pkt.flag & ACK) != 0 {
		if t.status != SYNSENT {
			return errors.New("status wrong, SYN ACK, " + t.Dump())
		}
		t.connest = nil
		t.status = EST
		err = t.send(ACK, []byte{})
		if err != nil { return }
		t.c_evout <- SYN
	}else{
		if t.status != CLOSED {
			return errors.New("status wrong, SYN, " + t.Dump())
		}
		t.status = SYNRCVD
		err = t.send(SYN | ACK, []byte{})
		if err != nil { return }
	}
	return
}

func (t *Tunnel) proc_fin (pkt *Packet) (err error) {
	t.recvseq += 1
	if (pkt.flag & ACK) != 0 {
		if t.status != FINWAIT {
			return errors.New("status wrong, FIN ACK, " + t.Dump())
		}else{ t.finwait = nil }
		t.status = TIMEWAIT
		err = t.send(ACK, []byte{})
		if err != nil { return }

		t.timewait = time.After(2 * time.Duration(TM_MSL) * time.Millisecond)
		t.c_evout <- FIN
	}else{
		switch t.status {
		case EST:
			t.status = LASTACK
			err = t.send(FIN | ACK, []byte{})
			if err != nil { return }
		case FINWAIT:
			t.finwait = nil
			t.status = TIMEWAIT
			err = t.send(ACK, []byte{})
			if err != nil { return }
			
			// wait 2*MSL to run close
			t.timewait = time.After(2 * time.Duration(TM_MSL) * time.Millisecond)
		default:
			return errors.New("status wrong, FIN, " + t.Dump())
		}
	}
	return
}

func (t *Tunnel) proc_sack(pkt *Packet) (err error) {
	var id int32
	var sendbuf PacketQueue
	buf := bytes.NewBuffer(pkt.content)

	binary.Read(buf, binary.BigEndian, &id)
	for _, p := range t.sendbuf {
		if p.seq == id {
			err = binary.Read(buf, binary.BigEndian, &id)
			if err == io.EOF {
				err = nil
				break
			}
			if err != nil { return }
		}else{ sendbuf = append(sendbuf, p) }
	}
	t.sendbuf = sendbuf

	t.sack_count += 1
	if t.sack_count > RETRANS_SACKCOUNT {
		t.resend(id, true)
	}

	return
}
