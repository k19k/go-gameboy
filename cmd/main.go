package main

import (
	"fmt"
	"gameboy"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage: %s rom\n", os.Args[0])
		return
	}

	quit := make(chan int)
	err := gameboy.Start(quit, os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err);
		return
	}

	<-quit
}
