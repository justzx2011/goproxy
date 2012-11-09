package udptunnel

import (
	"log"
	"net"
	"sync"
)

type UDPTunnel struct {
	udp net.UDPConn
	dispatcher map[int]UDPPair
	send_chan chan Packet
}

func DialTunnel(addr *net.UDPAddr) (tunnel *UDPTunnel, err error){
	tunnel = new(UDPTunnel)
	tunnel.udp, err = net.DialUDP("udp4", nil, addr)
	if err != nil { return }
	return
}

func (t *UDPTunnel)TunnelMain() {
	var sendpkt Packet
	var recvpkt Packet
	for {
		select {
		case sendpkt = <- t.send_chan:
			sendpkt.WriteTo(udp)
		case recvpkt.ReadFrom(udp):
			if recvpkt.id == 0 {

			}else{
				p := t.dispatcher[recvpkt.id]
				p.OnPacket(recvpkt)
			}
				
				
		}
	}
}

func (t *UDPTunnel) CreatePair() (pair *UDPPair, err error) {
	p = new(UDPPair)
	p.tunnel = t
	// require a id
	// make p.c
	return
}

type Socket struct {
	tunnel *UDPTunnel
	id uint16
	sendseq int32
	recvseq int32
	sendbuf []Packet
	recvbuf []byte
	c sync.Cond
}

func (p *UDPPair) OnPacket(pkt *Packet) (err error) {
	if p.flag | PSH {
		
	}

	if len(p.recvbuf) > 0 {
		append(p.recvbuf, pkt.content...)
		p.recvseq += recvpkt.content
		p.c.Signal()
		pkt := Packet{id=p.id, seq=p.sendseq, ack=p.recvseq, flag=PSH}
		p.tunnel.send_chan <- pkt
	}
}

func (p *UDPPair) Read(b []byte) (n int, err error) {
	for len(p.recvbuf) == 0 { p.c.Wait() }
	n = copy(b, p.recvbuf)
	p.recvbuf = p.recvbuf[n:]
	b = b[:n]
	return
}

func (p *UDPPair) Write(b []byte) (n int, err error) {
	var pkt Packet
	var buf []byte

	reader := bytes.Buffer(b)
	for buf = reader.Next(PACKETSIZE) {
		pkt.
	}
}