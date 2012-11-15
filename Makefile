### Makefile --- 

## Author: shell@shell-deb.shdiv.qizhitech.com
## Version: $Id: Makefile,v 0.0 2012/11/02 06:18:14 shell Exp $
## Keywords: 
## X-URL: 
TARGET=goproxy logger

all: clean build

build: $(TARGET)

install:
	install -d $(DESTDIR)/usr/bin/
	install -s goproxy $(DESTDIR)/usr/bin/
	install daemonized $(DESTDIR)/usr/bin/

clean:
	rm -f $(TARGET)

goproxy: goproxy.go
	go build -o $@ $^
	chmod 755 $@
	strip $@

logger: logger.go
	go build -o $@ $^
	chmod 755 $@
	strip $@

### Makefile ends here
