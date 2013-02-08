// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin freebsd linux netbsd openbsd

// DNS client: see RFC 1035.
// Has to be linked into package net for Dial.

// TODO(rsc):
//	Check periodically whether /etc/resolv.conf has changed.
//	Could potentially handle many outstanding lookups faster.
//	Could have a small cache.
//	Random UDP source port (net.Dial should do that for us).
//	Random request IDs.

package dns

import (
	"math/rand"
	"time"
	"net"
)

// Send a request on the connection and hope for a reply.
// Up to cfg.attempts attempts.
func exchange(cfg *dnsConfig, c net.Conn, name string, qtype uint16) (*dnsMsg, error) {
	if len(name) >= 256 {
		return nil, &DNSError{Err: "name too long", Name: name}
	}
	out := new(dnsMsg)
	out.id = uint16(rand.Int()) ^ uint16(time.Now().UnixNano())
	out.question = []dnsQuestion{
		{name, qtype, dnsClassINET},
	}
	out.recursion_desired = true
	msg, ok := out.Pack()
	if !ok {
		return nil, &DNSError{Err: "internal error - cannot pack message", Name: name}
	}

	for attempt := 0; attempt < cfg.attempts; attempt++ {
		n, err := c.Write(msg)
		if err != nil {
			return nil, err
		}

		if cfg.timeout == 0 {
			c.SetReadDeadline(time.Time{})
		} else {
			c.SetReadDeadline(time.Now().Add(time.Duration(cfg.timeout) * time.Second))
		}

		buf := make([]byte, 2000) // More than enough.
		n, err = c.Read(buf)
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Timeout() {
				continue
			}
			return nil, err
		}
		buf = buf[0:n]
		in := new(dnsMsg)
		if !in.Unpack(buf) || in.id != out.id {
			continue
		}
		return in, nil
	}
	var server string
	if a := c.RemoteAddr(); a != nil {
		server = a.String()
	}
	return nil, &DNSError{Err: "no answer from server", Name: name, Server: server, IsTimeout: true}
}

// Do a lookup for a single name, which must be rooted
// (otherwise answer will not find the answers).
func tryOneName(cfg *dnsConfig, name string, qtype uint16) (cname string, addrs []dnsRR, err error) {
	if len(cfg.servers) == 0 {
		return "", nil, &DNSError{Err: "no DNS servers", Name: name}
	}
	for i := 0; i < len(cfg.servers); i++ {
		// Calling Dial here is scary -- we have to be sure
		// not to dial a name that will require a DNS lookup,
		// or Dial will call back here to translate it.
		// The DNS config parser has already checked that
		// all the cfg.servers[i] are IP addresses, which
		// Dial will use without a DNS lookup.
		server := cfg.servers[i] + ":53"
		c, cerr := net.Dial("udp", server)
		if cerr != nil {
			err = cerr
			continue
		}
		msg, merr := exchange(cfg, c, name, qtype)
		c.Close()
		if merr != nil {
			err = merr
			continue
		}
		cname, addrs, err = answer(name, server, msg, qtype)
		if err == nil || err.(*DNSError).Err == noSuchHost {
			break
		}
		if cfg.CheckBlack(addrs) {
			
		}
	}
	return
}

func convertRR_A(records []dnsRR) []net.IP {
	addrs := make([]net.IP, len(records))
	for i, rr := range records {
		a := rr.(*dnsRR_A).A
		addrs[i] = net.IPv4(byte(a>>24), byte(a>>16), byte(a>>8), byte(a))
	}
	return addrs
}

func convertRR_AAAA(records []dnsRR) []net.IP {
	addrs := make([]net.IP, len(records))
	for i, rr := range records {
		a := make(net.IP, net.IPv6len)
		copy(a, rr.(*dnsRR_AAAA).AAAA[:])
		addrs[i] = a
	}
	return addrs
}

var cfg *dnsConfig

func LoadConfig(configfile string) (err error) {
	cfg, err = dnsReadConfig(configfile)
	return
}

func lookup(name string, qtype uint16) (cname string, addrs []dnsRR, err error) {
	if !isDomainName(name) {
		return name, nil, &DNSError{Err: "invalid domain name", Name: name}
	}
	// If name is rooted (trailing dot) or has enough dots,
	// try it by itself first.
	rooted := len(name) > 0 && name[len(name)-1] == '.'
	if rooted || count(name, '.') >= cfg.ndots {
		rname := name
		if !rooted {
			rname += "."
		}
		// Can try as ordinary name.
		cname, addrs, err = tryOneName(cfg, rname, qtype)
		if err == nil {
			return
		}
	}
	if rooted {
		return
	}

	// Otherwise, try suffixes.
	for i := 0; i < len(cfg.search); i++ {
		rname := name + "." + cfg.search[i]
		if rname[len(rname)-1] != '.' {
			rname += "."
		}
		cname, addrs, err = tryOneName(cfg, rname, qtype)
		if err == nil {
			return
		}
	}

	// Last ditch effort: try unsuffixed.
	rname := name
	if !rooted {
		rname += "."
	}
	cname, addrs, err = tryOneName(cfg, rname, qtype)
	if err == nil {
		return
	}
	return
}

func LookupIP(name string) (addrs []net.IP, err error) {
	// Use entries from /etc/hosts if possible.
	haddrs := lookupStaticHost(name)
	if len(haddrs) > 0 {
		for _, haddr := range haddrs {
			if ip := net.ParseIP(haddr); ip != nil {
				addrs = append(addrs, ip)
			}
		}
		if len(addrs) > 0 {
			return
		}
	}
	// TODO: test dns poison
	var records []dnsRR
	var cname string
	cname, records, err = lookup(name, dnsTypeA)
	if err != nil {
		return
	}
	addrs = convertRR_A(records)
	if cname != "" {
		name = cname
	}
	_, records, err = lookup(name, dnsTypeAAAA)
	if err != nil && len(addrs) > 0 {
		// Ignore error because A lookup succeeded.
		err = nil
	}
	if err != nil {
		return
	}
	addrs = append(addrs, convertRR_AAAA(records)...)
	return
}