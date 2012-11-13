package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func main () {
	// fmt.Println(uint8(0xff), ^uint8(0x00))
	var id uint16
	buf := bytes.NewBuffer([]byte{0x00, 0x01})
	binary.Read(buf, binary.BigEndian, &id)
	fmt.Println(id)
	err := binary.Read(buf, binary.BigEndian, &id)
	fmt.Println(err)
	fmt.Println(id)
}