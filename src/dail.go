package src

import (
	"../dns"
	"../sutils"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type IPList []net.IPNet

func ReadIPList(filename string) (iplist IPList, err error) {
	var f io.ReadCloser
	f, err = os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	if strings.HasSuffix(filename, ".gz") {
		f, err = gzip.NewReader(f)
		if err != nil {
			return
		}
	}

	err = sutils.ReadLines(f, func(line string) (err error) {
		addrs := strings.Split(strings.Trim(line, "\r\n "), " ")
		ipnet := net.IPNet{IP: net.ParseIP(addrs[0]), Mask: net.IPMask(net.ParseIP(addrs[1]))}
		iplist = append(iplist, ipnet)
		return
	})
	if err != nil {
		return
	}
	return
}

func (iplist IPList) Contain(ip net.IP) bool {
	for _, ipnet := range iplist {
		if ipnet.Contains(ip) {
			sutils.Debug(ipnet, "matches")
			return true
		}
	}
	return false
}

type IPEntry struct {
	t  time.Time
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
	for _, k := range dellist {
		delete(dc, k)
	}
	sutils.Info(len(dellist), "dnscache records deleted.")
	return
}

func (dc DNSCache) Lookup(hostname string) (ip net.IP, err error) {
	ipe, ok := dc[hostname]
	if ok {
		sutils.Debug("hostname", hostname, "cached")
		return ipe.ip, nil
	}

	addrs, err := dns.LookupIP(hostname)
	if err != nil {
		return
	}

	ip = addrs[0]
	ipe = new(IPEntry)
	ipe.ip = addrs[0]
	ipe.t = time.Now()

	if len(dc) > 256 {
		dc.free()
	}
	dnscache[hostname] = ipe
	return
}

var dnscache DNSCache = make(DNSCache, 0)

var blacklist IPList
var serveraddr string
var cryptWrapper func(net.Conn) (net.Conn, error) = nil
var username string
var password string

func InitDail(blackfile string, serveraddr_ string,
	cryptWrapper_ func(net.Conn) (net.Conn, error),
	username_ string, password_ string) {
	if blackfile != "" {
		var err error
		blacklist, err = ReadIPList(blackfile)
		sutils.Info("blacklist loaded,", len(blacklist), "record(s).")
		if err != nil {
			panic(err.Error())
		}
	}

	serveraddr = serveraddr_
	cryptWrapper = cryptWrapper_
	username = username_
	password = password_
	return
}

func Dail(hostname string, port uint16) (c net.Conn, err error) {
	var addr net.IP
	if blacklist == nil {
		return net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
	}
	addr = net.ParseIP(hostname)
	if addr == nil {
		addr, err = dnscache.Lookup(hostname)
		if err != nil {
			return
		}
	}
	switch {
	case blacklist.Contain(addr):
		sutils.Debug("ip", addr, "in black list.")
		return net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
	}
	return connect_qsocks(serveraddr, username, password, hostname, port)
}

func DialConn(network, addr string) (c net.Conn, err error) {
	addrs := strings.Split(addr, ":")
	hostname := addrs[0]
	port, err := strconv.Atoi(addrs[1])
	if err != nil {
		return
	}
	return Dail(hostname, uint16(port))
}
