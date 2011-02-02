package main

import (
	"fmt"
	"gameboy"
	"os"
	"path"
)

const (
	dotCmdName = ".gogb"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage: %s rom\n", os.Args[0])
		return
	}

	cfg := gameboy.Config{
		SaveDir: ".",
		Verbose: true,
		Debug:   true}

	home := os.Getenv("HOME")
	if len(home) > 0 {
		dir := path.Join(home, dotCmdName, "sav")
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		} else {
			cfg.SaveDir = dir
		}
	}

	quit := make(chan int)
	err := gameboy.Start(os.Args[1], cfg, quit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		return
	}

	<-quit
}
