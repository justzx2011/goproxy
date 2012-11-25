#!/usr/bin/python
# -*- coding: utf-8 -*-
'''
@date: 2012-11-26
@author: shell.xu
'''
import os, sys
from gevent import socket
import packet

TM_KEEPALIVE = 3600000

TIMEWAIT = 0

class Tunnel(object):

    def __init__(self, conn):
        self.conn = conn
        self.status = CLOSED
        self.seq_recv = 0
        self.q_recv = 0         # FIXME: what?

    def quit(self):
        pass                    # TODO:

    def on_packet(self, p):
        self.t_keep = TM_KEEPALIVE
        if p.flag & packet.ACKMASK == packet.RST:
            t.quit()
            return
        if t.status == TIMEWAIT:
            if p.flag & packet.ACKMASK == packet.SYN:
                t.quit()
                return
            elif p.flag & packet.ACKMASK == packet.FIN:
                t.send(packet.ACK, None)
                return
        # FIXME: round algorithm
        diff = p.seq - t.seq_recv
        if p.flag & packet.ACK != 0:
            self.recv_ack(p)
        if p.flag & packet.ACKMASK == packet.SACK:
            self.recv_sack(p)
            if diff > 0: t.send_sack()
            return
        if diff > 0:
            if len(p.content) != 0 or p.flag != packet.ACK:
                self.q_recv.push(p) # FIXME: q_recv? what type?
            self.send_sack()
            return
        pass                    # TODO: what next?

    def recv_ack(self, p):
        pass                    # TODO:

    def recv_sack(self, p):
        pass                    # TODO:

    def send_sack(self):
        pass                    # TODO:

def client_main(t):
    while True:
        buf = t.conn.recv(packet.Packet.MAXPACKET)
        if not buf: raise EOFError()
        p = packet.unpack(buf)
        t.on_packet(p)

def connect(addr):
    conn = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    t = Tunnel(conn)
    gevent.spawn(client_main, t)
    return t

class TunnelServer(object):
    pass
