package main

import (
	// "bytes"
	// "errors"
	"fmt"
	// "encoding/binary"
	// "io"
	// "log"
	// "net"
	"time"
	// "reflect"
	// "./sutils"
	// "./tunnel"
)

const (
	NETTICK = 1000 * 100 // nanosecond
	NETTICK_S = 1000 * 1000 * 1000 / NETTICK
	NETTICK_M = 1000 * 1000 / NETTICK
)

func get_nettick (ti time.Time) (int32) {
	t := ti.UnixNano()/NETTICK
	// t := ti.Second()*NETTICK_S + ti.Nanosecond()/NETTICK
	return int32(t)
}

func main () {
	v := int64(128*256*256*256-1)
	fmt.Println(int32(v+1))
}