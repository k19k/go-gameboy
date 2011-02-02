package main

import (
	"flag"
	"fmt"
	"gameboy"
	"os"
	"path"
)

const (
	dotCmdName = ".gogb"
)

var config gameboy.Config

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		fmt.Printf("usage: %s [flags] rom\n", os.Args[0])
		flag.PrintDefaults()
		return
	}

	if err := os.MkdirAll(config.SaveDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		return
	}

	quit := make(chan int)
	err := gameboy.Start(args[0], config, quit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		return
	}

	<-quit
}

func init() {
	flag.StringVar(&config.SaveDir, "savedir",
		path.Join(os.Getenv("HOME"), dotCmdName, "sav"),
		"where to store save files")
	flag.BoolVar(&config.Verbose, "v", false, "print verbose output")
	flag.BoolVar(&config.Debug, "debug", false, "print debug messages")
}
