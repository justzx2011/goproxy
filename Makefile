### Makefile --- 

## Author: shell@shell-deb.shdiv.qizhitech.com
## Version: $Id: Makefile,v 0.0 2012/11/02 06:18:14 shell Exp $
## Keywords: 
## X-URL: 
TARGET=goproxy
DEBUGOPT=--loglevel INFO
# --keyfile keyfile
# DEBUGSRV=--logfile buf:/tmp/server.log
# DEBUGCLI=--logfile buf:/tmp/client.log

all: clean build

build: $(TARGET)

install:
	install -d $(DESTDIR)/usr/bin/
	install -s goproxy $(DESTDIR)/usr/bin/
	install daemonized $(DESTDIR)/usr/bin/

clean:
	rm -f $(TARGET)

test:
	go test ./tunnel

server: goproxy
	rm -f /tmp/server.log /tmp/srv.log
	./goproxy --mode server --listen :8899 $(DEBUGOPT) $(DEBUGSRV)
# > /tmp/srv.log

client: goproxy
	rm -f /tmp/client.log /tmp/cli.log
	./goproxy --mode client --listen :1081 $(DEBUGOPT) $(DEBUGCLI) debox:8899
# > /tmp/cli.log

goproxy: goproxy.go
	go build -o $@ $^
	strip $@
	chmod 755 $@

echogo: echo.go
	go build -o $@ $^
	# strip $@
	chmod 755 $@

cligo: cli.go
	go build -o $@ $^
	# strip $@
	chmod 755 $@

testlog: logger
	rm -f *.log
	./logger --listen :4455 --loglevel DEBUG

logger: logger.go
	go build -o $@ $^
	strip $@
	chmod 755 $@

### Makefile ends here
