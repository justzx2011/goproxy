package qsocks

// import (
// 	"net"
// )

// func DNS(hostname string) (buf []byte, err error) {
// 	size := uint16(1 + len(hostname))
// 	buf := make([]byte, size)
// 	cur = fillString(cur, hostname)
// 	return
// }

// func getDNS(conn net.Conn) (hostname string, err error) {
// 	return getString(conn)
// }

// func Answer(addrs []net.IP) (buf []byte, err error) {
	
// }

// func getAnswer(conn net.Conn) (addrs []net.IP, err error) {
// 	return
// }

// type DnsQuiz struct {
// 	name string
// 	answer chan net.IP
// }

// var ch_dns chan string

// func gmain(srv string) {
// 	var conn net.Conn

// 	conn = net.Dail("tcp", srv)
// 	for {
// 		quiz := <- ch_dns
// 		bufDNS := DNS(quiz.name)
// 		_, err = conn.Write(bufDNS)
// 		if err != nil {
// 			sutils.Err(err)
// 			close(quiz.answer)
// 			continue
// 		}

// 	}
// }

// func dnsinit(addr string) {
// 	ch_dns = make(chan string, 0)
// 	go gmain(addr)
// }

// func lookup(hostname string) (addrs []net.IP, err error) {
	
// 	return
// }