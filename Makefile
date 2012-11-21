### Makefile --- 

## Author: shell@shell-deb.shdiv.qizhitech.com
## Version: $Id: Makefile,v 0.0 2012/11/02 06:18:14 shell Exp $
## Keywords: 
## X-URL: 
TARGET=goproxy
DEBUGOPT=--loglevel WARNING
# DEBUGSRV=--logfile buf:server.log
# DEBUGCLI=--logfile buf:client.log

all: clean build

build: $(TARGET)

testlog: logger
	rm -f *.log
	./logger --listen :4455 --loglevel DEBUG

server: goproxy
	./goproxy --mode udpsrv --listen :8899 $(DEBUGOPT) $(DEBUGSRV)

client: goproxy
	./goproxy --mode udpcli --listen :1081 $(DEBUGOPT) $(DEBUGCLI) localhost:8899

install:
	install -d $(DESTDIR)/usr/bin/
	install -s goproxy $(DESTDIR)/usr/bin/
	install daemonized $(DESTDIR)/usr/bin/

clean:
	rm -f $(TARGET)

goproxy: goproxy.go
	go build -o $@ $^
	strip $@
	chmod 755 $@

logger: logger.go
	go build -o $@ $^
	strip $@
	chmod 755 $@

### Makefile ends here
