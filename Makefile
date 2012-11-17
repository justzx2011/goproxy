### Makefile --- 

## Author: shell@shell-deb.shdiv.qizhitech.com
## Version: $Id: Makefile,v 0.0 2012/11/02 06:18:14 shell Exp $
## Keywords: 
## X-URL: 
TARGET=goproxy logger

all: clean build

build: $(TARGET)

test: goproxy logger
	./logger --listen :4455 --loglevel DEBUG &
	echo $!
	./goproxy --mode udpsrv --listen :8899 --logfile localhost:4455 --loglevel DEBUG &
	echo $!
	./goproxy --mode udpcli --listen :1081 --logfile localhost:4455 --loglevel DEBUG localhost:8899 &
	echo $!

install:
	install -d $(DESTDIR)/usr/bin/
	install -s goproxy $(DESTDIR)/usr/bin/
	install daemonized $(DESTDIR)/usr/bin/

clean:
	rm -f $(TARGET)
	rm -f *.log

goproxy: goproxy.go
	go build -o $@ $^
	strip $@
	chmod 755 $@

logger: logger.go
	go build -o $@ $^
	strip $@
	chmod 755 $@

### Makefile ends here
