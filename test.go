package main

import (
	"fmt"
	// "container/heap"
)

type PacketHeap []byte

func (ph PacketHeap) Len() int { return len(ph) }

func (ph PacketHeap) Less(i, j int) bool {
	return ph[i] < ph[j]
}

func (ph PacketHeap) Swap(i, j int) {
	ph[i], ph[j] = ph[j], ph[i]
}

func (ph *PacketHeap) Push(x interface{}) {
	*ph = append(*ph, x.(byte))
}

func (ph *PacketHeap) Pop() interface{} {
	x := (*ph)[len(*ph)-1]
	*ph = (*ph)[:len(*ph)-1]
	return x
}

func main () {
	// var ph PacketHeap
	// heap.Init(ph)

	// heap.Push(&ph, byte(0x02))
	// fmt.Println(ph)

	// heap.Push(&ph, byte(0x01))
	// fmt.Println(ph)
	
	// fmt.Println(heap.Pop(&ph))
	// fmt.Println(ph)

	// fmt.Println(heap.Pop(&ph))
	// fmt.Println(ph)

	// for i := 0; i < 1; i++ {
	// 	fmt.Println(i)
	// }

	// var c chan int = make(chan int, 10000)
	// fmt.Println(len(c))
	// c <- 1
	// fmt.Println(len(c))

	fmt.Println(uint8(0xff), ^uint8(0x00))
}