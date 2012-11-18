#!/usr/bin/python
# -*- coding: utf-8 -*-
'''
@date: 2012-11-14
@author: shell.xu
'''
import sys, gevent, logging
from urlparse import urlparse
from gevent import socket, pool
import http, socks

c = socks.socks5(('debox', 1081))(socket.socket)
def download(uri):
    url = urlparse(uri)

    r = (url.netloc or url.path).split(':', 1)
    if len(r) > 1: port = int(r[1])
    else: port = 443 if url.scheme.lower() == 'https' else 80
    hostname, port = r[0], port
    uri = url.path + ('?'+url.query if url.query else '')

    req = http.request_http(uri)
    res = http.http_client(req, (hostname, port), c)
    return res.read_body()


def main():
    url = 'http://www.douban.com/'
    d = download(url)
    counter = [0, 0, 0, 0]

    def writest(ch):
        sys.stdout.write(
            '%d/%d/%d/%d = %f/%f/%f%s' % (
                counter[0], counter[1], counter[2], counter[3],
                float(counter[0])/float(counter[3]), float(counter[1])/float(counter[3]),
                float(counter[2])/float(counter[3]), ch))
        
    def tester():
        counter[3] += 1
        try:
            e = download(url)
            if d == e: counter[0] += 1
            else: counter[1] += 1
        except Exception, e: counter[2] += 1
        writest('\r')

    p = pool.Pool(200)
    for i in xrange(2000): p.spawn(tester)
    p.join()
    writest('\n')

if __name__ == '__main__': main()
