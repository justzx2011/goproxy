package main

import (
	// "fmt"
	"log"
	// "net"
	"./sutils"
)

func main () {
	lv, err := sutils.GetLevelByName("DEBUG")
	err = sutils.SetupLog(":4455", lv)
	if err != nil { log.Println(err) }
	t := sutils.NewLogger("test")
	t.Info("OK", 1, 2)
}