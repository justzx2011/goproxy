#!/usr/bin/python
# -*- coding: utf-8 -*-
'''
@date: 2012-11-26
@author: shell.xu
'''
import zlib, struct

MSS = 1024

DAT = 0
SYN = 1
FIN = 2
RST = 3
PST = 4
SACK = 5
ACK = 0x80
ACKMASK = ~ACK

class Packet(object):
    HEADERSIZE = 25
    MAXPACKET = HEADERSIZE + MSS

    def __init__(self, content):
        self.content = content

    def pack(self):
        b1 = struct.pack('>BIIIIIH', self.flag, self.window, self.seq, self.ack,
                         self.sndtime, self.acktime, len(self.content))
        crc = zlib.crc32(b1)
        if len(self.content) > 0: crc = zlib.crc32(self.content, crc)
        b2 = struct.pack('>H', crc & 0xffff)
        return b1 + b2 + self.content

def unpack(buf):
    flag, window, seq, ack, sndtime, acktime, size, crc1 = \
        struct.unpack(">BIIIIIHH", buf[:HEADERSIZE])
    if len(buf) - HEADERSIZE != size: raise Exception("packet not match")
    crc = zlib.crc32(buf[:HEADERSIZE-4])
    if size > 0: crc = zlib.crc32(buf[HEADERSIZE:], crc)
    if crc1 != crc & 0xffff: raise Exception("crc not match")
    p = Packet(buf[HEADERSIZE:])
    p.flag, p.window, p.seq, p.ack, p.sndtime, p.acktime = \
        flag, window, seq, ack, sndtime, acktime
    return p
