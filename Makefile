### Makefile --- 

## Author: shell@shell-deb.shdiv.qizhitech.com
## Version: $Id: Makefile,v 0.0 2012/11/02 06:18:14 shell Exp $
## Keywords: 
## X-URL: 
TARGET=socksv5 tcp_cli tcp_srv

all: $(TARGET)

clean:
	rm -f $(TARGET)

socksv5: socksv5.go
	go build $^
	chmod 755 $@
	strip $@

tcp_cli: tcp_cli.go
	go build $^
	chmod 755 $@
	strip $@

tcp_srv: tcp_srv.go
	go build $^
	chmod 755 $@
	strip $@

### Makefile ends here
