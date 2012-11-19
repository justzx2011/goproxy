package main

import (
	// "bytes"
	"errors"
	"fmt"
	// "encoding/binary"
	// "io"
	// "log"
	// "net"
	// "./sutils"
	// "./tunnel"
)

// func main () {
// 	b1 := []int8{0x01, 0x02, 0x03, 0x14, 0x18}
// 	b2 := []byte{0x12, 0x14}
// 	buf := bytes.NewBuffer(b2)

// 	var err error
// 	var id int8
// 	var i int = 0
// 	var b3 []int8

// 	binary.Read(buf, binary.BigEndian, &id)
// 	for i < len(b1) {
// 		v := b1[i]
// 		fmt.Println("loop", i, v, id)
// 		df := v - id

// 		switch {
// 		case df == 0:
// 			fmt.Println("hit id", id)
// 			i += 1
// 		case df < 0:
// 			fmt.Println("append", v)
// 			b3 = append(b3, v)
// 			i += 1
// 		}

// 		if df >= 0 {
// 			err = binary.Read(buf, binary.BigEndian, &id)
// 			if err == io.EOF {
// 				err = nil
// 				break
// 			}
// 			if err != nil { return }
// 		}
// 	}
// 	fmt.Println(i, b1[i:], b3)
// 	if i < len(b1) {
// 		b3 = append(b3, b1[i:]...)
// 	}

// 	fmt.Println(b1, b2, b3)
// 	return
// }

func f1(){
	// panic("def")
}

func f2() (err error) {
	defer func () {
		if x := recover(); x != nil {
			err = errors.New(x.(string))
		}
	}()
	f1()
	return errors.New("abc")
}

func main () {
	fmt.Println(f2())
}