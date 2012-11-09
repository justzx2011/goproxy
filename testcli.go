package main

import (
	"fmt"
)

func main () {
	var f func ()
	fmt.Println(f)
	if f == nil {
		fmt.Println("ok")
	}else{
		fmt.Println("not ok")
	}
}