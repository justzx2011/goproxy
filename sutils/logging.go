package sutils

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

const TIMEFMT = "2006-01-02T15:04:05.000"

var lvname = map[int]string {
	0: "EMERG", 1: "ALERT", 2: "CRIT",
	3: "ERROR", 4: "WARNING", 5: "NOTICE",
	6: "INFO", 7: "DEBUG",
}

const (
	LOG_EMERG = iota
	LOG_ALERT
	LOG_CRIT
	LOG_ERR
	LOG_WARNING
	LOG_NOTICE
	LOG_INFO
	LOG_DEBUG
)

var console bool
var loglv int
var logconn *net.UDPConn
var facility int
var output *os.File
var outchan chan string
var hostname string

func GetLevelByName(name string) (lv int, err error) {
	for k, v := range lvname {
		if v == name { return k, nil }
	}
	return -1, fmt.Errorf("unknown loglevel")
}

func SetupLog(logfile string, loglevel int, f int) (err error) {
	// logfile is empty string: use console
	// udp address: syslog format, use f as facility
	// buf:filename: buffered file
	// filename: output to file
	loglv = loglevel
	facility = f

	if len(logfile) == 0 {
		console = true
		return
	}

	addr, e := net.ResolveUDPAddr("udp", logfile)
	if e == nil {
		logconn, err = net.DialUDP("udp", nil, addr)
		if err != nil { return }
		hostname, err = os.Hostname()
		return
	}

	if strings.HasPrefix(logfile, "buf:") {
		logfile = logfile[4:]
	}
	output, err = os.OpenFile(logfile, os.O_WRONLY | os.O_APPEND | os.O_CREATE, 0644)
	if strings.HasPrefix(logfile, "buf:") {
		outchan = make(chan string, 1000)
		go outchan_main()
	}
	return
}

func outchan_main () {
	bufout := bufio.NewWriterSize(output, 512)
	bufout.Write([]byte("buffered\n"))
	for {
		s := <- outchan
		bufout.WriteString(s)
	}
}

func WriteLog(name string, lv int, a []interface{}) {
	switch{
	case console:
		h := fmt.Sprintf("%s %s[%s] ", time.Now().Format(TIMEFMT), name, lvname[lv])
		fmt.Print(h + fmt.Sprintln(a...))
	case logconn != nil:
		// <facility * 8 + pri>version timestamp hostname app-name procid msgid
		// <facility * 8 + pri>timestamp hostname procid msgid
		h := fmt.Sprintf("<%d>%s %s %d %s[]: ", facility * 8 + lv,
			time.Now().Format(TIMEFMT), hostname, os.Getpid(), name)
		logconn.Write([]byte(h + fmt.Sprintln(a...) + "\n"))
	case outchan != nil:
		h := fmt.Sprintf("%s %s[%s] ", time.Now().Format(TIMEFMT), name, lvname[lv])
		outchan <- (h + fmt.Sprintln(a...))
	case output != nil:
		h := fmt.Sprintf("%s %s[%s] ", time.Now().Format(TIMEFMT), name, lvname[lv])
		output.Write([]byte(h + fmt.Sprintln(a...)))
	}
}

type Logger struct {
	name string
}

func NewLogger(name string) (logger *Logger) {
	return &Logger{name}
}

func (l *Logger) Alert(a ...interface{}) {
	if loglv < LOG_ALERT { return }
	WriteLog(l.name, LOG_ALERT, a)
}

func (l *Logger) Crit(a ...interface{}) {
	if loglv < LOG_CRIT { return }
	WriteLog(l.name, LOG_CRIT, a)
}

func (l *Logger) Debug(a ...interface{}) {
	if loglv < LOG_DEBUG { return }
	WriteLog(l.name, LOG_DEBUG, a)
}

func (l *Logger) Emerg(a ...interface{}) {
	if loglv < LOG_EMERG { return }
	WriteLog(l.name, LOG_EMERG, a)
}

func (l *Logger) Err(a ...interface{}) {
	if loglv < LOG_ERR { return }
	WriteLog(l.name, LOG_ERR, a)
}

func (l *Logger) Info(a ...interface{}) {
	if loglv < LOG_INFO { return }
	WriteLog(l.name, LOG_INFO, a)
}

func (l *Logger) Notice(a ...interface{}) {
	if loglv < LOG_NOTICE { return }
	WriteLog(l.name, LOG_NOTICE, a)
}

func (l *Logger) Warning(a ...interface{}) {
	if loglv < LOG_WARNING { return }
	WriteLog(l.name, LOG_WARNING, a)
}

func Alert(a ...interface{}) {
	if loglv < LOG_ALERT { return }
	WriteLog("", LOG_ALERT, a)
}

func Crit(a ...interface{}) {
	if loglv < LOG_CRIT { return }
	WriteLog("", LOG_CRIT, a)
}

func Debug(a ...interface{}) {
	if loglv < LOG_DEBUG { return }
	WriteLog("", LOG_DEBUG, a)
}

func Emerg(a ...interface{}) {
	if loglv < LOG_EMERG { return }
	WriteLog("", LOG_EMERG, a)
}

func Err(a ...interface{}) {
	if loglv < LOG_ERR { return }
	WriteLog("", LOG_ERR, a)
}

func Info(a ...interface{}) {
	if loglv < LOG_INFO { return }
	WriteLog("", LOG_INFO, a)
}

func Notice(a ...interface{}) {
	if loglv < LOG_NOTICE { return }
	WriteLog("", LOG_NOTICE, a)
}

func Warning(a ...interface{}) {
	if loglv < LOG_WARNING { return }
	WriteLog("", LOG_WARNING, a)
}
