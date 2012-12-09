#!/usr/bin/python
# -*- coding: utf-8 -*-
'''
@date: 2012-11-14
@author: shell.xu
'''
import sys, md5, gevent, logging
from getopt import getopt
from urlparse import urlparse
from gevent import socket, pool
import http, socks

c = socks.socks5(('localhost', 1081))(socket.socket)
# c = socket.socket
def download(uri, downfull=False):
    url = urlparse(uri)

    r = (url.netloc or url.path).split(':', 1)
    if len(r) > 1: port = int(r[1])
    else: port = 443 if url.scheme.lower() == 'https' else 80
    hostname, port = r[0], port
    uri = url.path + ('?'+url.query if url.query else '')

    req = http.request_http(uri)
    req.set_header("Host", url.hostname)
    res = http.http_client(req, (hostname, port), c)

    if downfull: return res.read_body()

    m = md5.new()
    cnt = 0
    for d in res.read_chunk(res.stream):
        cnt += len(d)
        m.update(d)
    sys.stdout.write('done\n')
    return m.hexdigest()

def doloop(url, loops):
    d = download(url, False)
    counter = [0, 0, 0, 0]

    def writest():
        sys.stdout.write(
            '%d/%d/%d/%d = %f/%f/%f\n' % (
                counter[0], counter[1], counter[2], loops,
                float(counter[0])/loops, float(counter[1])/loops,
                float(counter[2])/loops))
        
    def tester():
        try:
            e = download(url, False)
            if d == e: counter[0] += 1
            else:
                sys.stdout.write("%s != %s" % (d, e))
                counter[1] += 1
        except Exception, e: counter[2] += 1
        writest()

    p = pool.Pool(300)
    for i in xrange(loops): p.spawn(tester)
    try: p.join()
    finally: writest()

def initlog(lv, logfile=None):
    rootlog = logging.getLogger()
    if logfile: handler = logging.FileHandler(logfile)
    else: handler = logging.StreamHandler()
    handler.setFormatter(
        logging.Formatter(
            '%(asctime)s,%(msecs)03d (%(process)d)%(name)s[%(levelname)s]: %(message)s',
            '%H:%M:%S'))
    rootlog.addHandler(handler)
    rootlog.setLevel(lv)

def main():
    initlog(logging.INFO)
    optlist, args = getopt(sys.argv[1:], "b:o:t")
    optdict = dict(optlist)
    if not args: url = 'http://localhost/'
    else: url = args[0]
    if '-o' in optdict:
        with open(optdict['-o'], 'wb') as fo:
            fo.write(download(url, True))
    elif '-t' in optdict: print download(url, False)
    elif '-b': doloop(url, int(optdict['-b']))

if __name__ == '__main__': main()
