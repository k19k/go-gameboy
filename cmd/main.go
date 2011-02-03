// Copyright 2011 Kevin Bulusek. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"gameboy"
	"os"
	"os/signal"
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

	// The bounds are arbitrary, but seem more than reasonable.
	if config.Scale < 1 || config.Scale > 6 {
		fmt.Printf("unlikely scaling factor: %dx\n", config.Scale)
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

	for {
		select {
		case <-quit:
			return
		case sig := <-signal.Incoming:
			interrupt := true
			if id, ok := sig.(signal.UnixSignal); ok {
				if id != 2 { // SIGINT
					interrupt = false
				}
			}
			if interrupt {
				if config.Verbose {
					fmt.Printf("Caught %v, "+
						"cleaning up...\n",
						sig)
				}
				quit <- 1 // notify gameboy
				<-quit    // wait for response
				return
			}
		}
	}
}

func init() {
	flag.StringVar(&config.SaveDir, "savedir",
		path.Join(os.Getenv("HOME"), dotCmdName, "sav"),
		"where to store save files")
	flag.BoolVar(&config.Verbose, "v", false, "print verbose output")
	flag.BoolVar(&config.Debug, "debug", false, "print debug messages")
	flag.IntVar(&config.Scale, "scale", 2, "display scaling factor")
}
