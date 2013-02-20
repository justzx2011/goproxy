package main

import (
	"fmt"
	"compress/gzip"
	"io"
	"net"
	"os"
	"strings"
	"time"
	"./dns"
	"./qsocks"
	"./sutils"
)

type IPList []net.IPNet

func ReadIPList(filename string) (iplist IPList, err error) {
	var f io.Reader
	f, err = os.Open(filename)
	if err != nil { return }
	if strings.HasSuffix(filename, ".gz") {
		f, err = gzip.NewReader(f)
		if err != nil { return }
	}
	err = sutils.ReadLines(f, func (line string) (err error){
		addrs := strings.Split(strings.Trim(line, "\r\n "), " ")
		ipnet := net.IPNet{IP: net.ParseIP(addrs[0]), Mask: net.IPMask(net.ParseIP(addrs[1]))}
		iplist = append(iplist, ipnet)
		return
	})
	if err != nil { return }
	return
}

func (iplist IPList)Contain(ip net.IP) (bool) {
	for _, ipnet := range iplist {
		if ipnet.Contains(ip) {
			sutils.Debug(ipnet, "matches")
			return true
		}
	}
	return false
}

type IPEntry struct {
	t time.Time
	ip net.IP
}

type DNSCache map[string]*IPEntry

func (dc DNSCache) free() {
	var dellist []string
	n := time.Now()
	for k, v := range dc {
		if n.Sub(v.t).Seconds() > 300 {
			dellist = append(dellist, k)
		}
	}
	for _, k := range dellist { delete(dc, k) }
	return
}

func (dc DNSCache) Lookup(hostname string) (ip net.IP, err error) {
	ipe, ok := dc[hostname]
	if ok {
		sutils.Debug("hostname", hostname, "cached")
		return ipe.ip, nil
	}

	addrs, err := dns.LookupIP(hostname)
	if err != nil { return }

	ip = addrs[0]
	ipe = new(IPEntry)
	ipe.ip = addrs[0]
	ipe.t = time.Now()

	if len(dc) > 256 { dc.free() }
	dnscache[hostname] = ipe
	return 
}

var dnscache DNSCache = make(DNSCache, 0)

var blacklist IPList

func init_dail() {
	if blackfile != "" {
		var err error
		blacklist, err = ReadIPList(blackfile)
		sutils.Info("blacklist loaded,", len(blacklist), "record(s).")
		if err != nil { panic(err.Error()) }
	}

	err := dns.LoadConfig("resolv.conf")
	if err == nil { return }
	err = dns.LoadConfig("/etc/goproxy/resolv.conf")
	if err != nil { panic(err.Error()) }
	return
}

var serveraddr string

func connect_qsocks(hostname string, port uint16) (conn net.Conn, err error) {
	conn, err = net.Dial("tcp", serveraddr)
	if err != nil { return }

	if cryptWrapper != nil {
		conn, err = cryptWrapper(conn)
		if err != nil { return }
	}

	bufAuth, err := qsocks.Auth(username, password)
	if err != nil { return }
	_, err = conn.Write(bufAuth)
	if err != nil { return }

	bufConn, err := qsocks.Conn(hostname, port)
	if err != nil { return }
	_, err = conn.Write(bufConn)
	if err != nil { return }

	res, err := qsocks.RecvResponse(conn)
	if err != nil { return }
	if res != 0 { return nil, fmt.Errorf("qsocks response %d", res) }
	return
}

func connect_direct(hostname string, port uint16) (conn net.Conn, err error) {
	return net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
}

func dail(hostname string, port uint16) (c net.Conn, err error) {
	var addr net.IP
	if blacklist == nil {
		return connect_direct(hostname, port)
	}
	addr = net.ParseIP(hostname)
	if addr == nil {
		addr, err = dnscache.Lookup(hostname)
		// addr, err = cached_lookup(hostname)
		if err != nil { return }
	}
	switch {
	case blacklist.Contain(addr):
		sutils.Debug("ip", addr, "in black list.")
		return connect_direct(hostname, port)
	}
	return connect_qsocks(hostname, port)
}
