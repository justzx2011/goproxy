#!/usr/bin/python
# -*- coding: utf-8 -*-
'''
@date: 2012-11-14
@author: shell.xu
'''
import re, os, sys, socket
from datetime import datetime

serverty = {
    0: "EMERGENCY", 1: "ALERT", 2: "CRITICAL",
    3: "ERROR", 4: "WARNING", 5: "NOTICE",
    6: "INFO", 7: "DEBUG"
}

def writebuf(buf):
    for ti, data in buf:
        m = syslog_re.match(data.strip())
        if not m:
            print data
            continue
        h = m.group(1)
        s = serverty[int(h)%8]
        src = m.group(2).replace(':', '_')
        with open(src+'.log', 'a') as fo:
            fo.write('%s %s %s\n' % (ti, s, m.group(3)))

syslog_re = re.compile("<(\d+)>.*: \[(.*)\] (.*)")
def main():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.bind(("", 4455))
    buf = []

    try:
        while True:
            data, addr = sock.recvfrom(1024)
            buf.append((datetime.now(), data))
    finally: writebuf(buf)

if __name__ == '__main__': main()
