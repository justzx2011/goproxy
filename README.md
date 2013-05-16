# goproxy #

goproxy是基于go写的隧道代理服务器。主要分为两个部分，客户端和服务器端。客户端使用http或socks5协议向其他程序提供一个标准代理。当客户端接受请求后，会加密连接服务器端，请求服务器端连接目标。双方通过预共享的密钥和加密算法通讯，通过用户名/密码验证身份。

goproxy拥有众多参数，以下为参数解释。

* mode: 工作在何种模式下。
* cipher: 何种加密算法。目前支持aes/des/tripledes/rc4，默认aes。
* keyfile: 密钥文件。
* listen: 所监听的端口。默认:5233。
* username: 连接到服务器的用户名（仅在客户端模式有效）。
* password: 连接到服务器的密码（仅在客户端模式有效）。
* passfile: 验证客户端身份的密码文件（仅在服务端模式有效）。
* black: 黑名单文件。
* logfile: 日志文件。默认输出到console。
* loglevel: 日志级别。支持EMERG/ALERT/CRIT/ERROR/WARNING/NOTICE/INFO/DEBUG，默认WARNING。

# server模式 #

服务器模式一般需要定义密码文件以验证客户端身份。如果没有定义，则允许匿名使用。

## 密码文件 ##

密码文件使用文本格式，每个用户一行，以:分割。第一个:前的为用户名，后面直到换行为止是密码。因此用户名中不得带有:。

# client模式 #

客户端模式提供socks5协议的代理，代理端口在listen中指定。启动时需要在第一个参数指定一个服务端。

## 黑名单文件 ##

黑名单文件是一个路由文件，其中列出的子网将不会由服务器端代理，而是直接连接。这通常用于部分IP不希望通过服务器端的时候。

黑名单文件使用文本格式，每个子网一行。行内以空格分割，第一段为IP地址，第二段为子网掩码。允许使用gzip压缩，后缀名必须为gz，可以直接读取。routes.list.gz为样例。

## dns配置 ##

dns是goproxy中很特殊的一个功能。由于代理经常接到连接某域名的指令，因此为了进行ip匹配，需要先进行dns查询。为了某些特殊原因，goproxy将go自带的dns做了修改。

系统会首先尝试读取本目录的resolv.conf，然后读取/etc/goproxy/resolv.conf。该文件支持一般resolv.conf的所有配置，但是额外多出一项，blackip。

如果blackip有指定，那么当dns查询结果为blackip所指定的ip时，结果丢弃，等待下一个响应包的返回。这个行为可以很大程度上抵御dns污染。

源码中附带了一个resolv.conf，一般可以直接使用。

# httproxy模式 #

httproxy模式提供http协议的代理。但是由于实现的不好，目前不推荐使用。建议使用polipo+client模式。

# 启动脚本用法 #

## daemonized ##

这是一个python写的小脚本，目的是用于转换为系统服务，并监视goproxy的执行。go本身不适合做daemonized的工作，因此监控程序正常执行和重启的工作是由python来完成的。

* -f: 前台执行。
* -l: log文件。
* -h: help信息。
* -p: pid文件。检测pid文件是否存在，如果存在且pid文件指名的进程exe为执行程序，则不继续执行。

其余参数为要启动的程序和参数。daemonized脚本可以用于大部分程序的daemonized化和监控。

## init脚本 ##

在debian目录下有个默认的init脚本，负责将goproxy封装为服务。

## 配置和路径 ##

默认情况下，init脚本读取/etc/default/goproxy作为配置。刚安装的时候，RUNDAEMON关闭，直到配置完成后改为1，goproxy才可以启动。

DAEMON_OPTS里面需要指名运行goproxy所需的参数。注意goproxy自身不算参数，不需要写在里面。

系统自带的black文件在/usr/share/goproxy/routes.list.gz，但是如果需要用，必须在DAEMON_OPTS中以参数的形式显式指定。

## key文件的生成 ##

可以使用以下语句生成。文件生成后，在服务器端和客户端使用keyfile指定即可。

	dd if=/dev/random of=key count=32 bs=1
	chmod 400 key

其中aes需要32字节的随机数，des/tripledes需要24字节，rc4需要16字节。

## 配置样例 ##

服务器端/etc/default/goproxy下。

	RUNDAEMON=1
	DAEMON_OPTS="-mode server -keyfile=/etc/goproxy/key -passfile=/etc/goproxy/users.pwd"

客户端/etc/default/goproxy下。

	RUNDAEMON=1
	DAEMON_OPTS="-mode client -keyfile=/etc/goproxy/key -black=/usr/share/goproxy/routes.list.gz -username=usr -password=pwd srv:5233

客户端/etc/polipo/config。

	proxyPort = 8118 # 或者你希望的其他端口
	allowedClients = 127.0.0.1 # 按照需要修改
	socksParentProxy = "localhost:5233"
	socksProxyType = socks5

	chunkHighMark = 819200 # 不需要缓存
	objectHighMark = 128

# TODO #

goproxy解决了加密和SSL握手容易识别的问题，但是连接特性和流量特性依然十分明显。下一步考虑做连接复用和流量混淆。
