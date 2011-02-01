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

	ch, err := gameboy.Start(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err);
		return
	}

	<-ch
}
