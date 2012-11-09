package main

func UdpServer(addr string, handler func (net.Conn) (error)) (err error) {
	tcpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil { return }
	listener, err := net.ListenUDP("udp", tcpAddr)
	if err != nil { return }
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
			continue
		}
		go func () {
			e := handler(conn)
			if e != nil { log.Println(e.Error()) }
		} ()
	}
	return
}

func main () {
	UdpServer(":1111", func (conn net.Conn) (err error))
	
}