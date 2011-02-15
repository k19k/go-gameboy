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
	dotCmdName = ".go-gameboy"
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

	err := make(chan interface{})
	out := make(chan int)
	go gameboy.Start(args[0], config, out, err)

	for wait(out, err) {
		// keep waiting (do nothing)
	}
}

func wait(out chan int, error chan interface{}) bool {
	select {
	case err := <-error:
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		}
		return false
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
			out <- 1 // notify gameboy
		}
	}
	return true
}

func init() {
	flag.StringVar(&config.SaveDir, "savedir",
		path.Join(os.Getenv("HOME"), dotCmdName, "sav"),
		"where to store save files")
	flag.BoolVar(&config.Verbose, "v", false, "print verbose output")
	flag.BoolVar(&config.Debug, "debug", false, "print debug messages")
	flag.IntVar(&config.Scale, "scale", 2, "display scaling factor")
	flag.IntVar(&config.AudioFreq, "freq", 48000, "audio rate")
	flag.IntVar(&config.AudioBuffers, "nbuf", 4, "audio buffers")
	flag.StringVar(&config.AudioDriver, "adev", "",
		"libao driver name (e.g. pulse, alsa)")
	flag.BoolVar(&config.Fullscreen, "fs", false, "run in fullscreen mode")
	flag.IntVar(&config.Joystick, "joystick", 0, "which joystick to use")
	flag.IntVar(&config.JoyButtonA, "joy-a", 1, "joystick A button")
	flag.IntVar(&config.JoyButtonB, "joy-b", 0, "joystick B button")
	flag.IntVar(&config.JoyButtonStart, "joy-start", 6, "joystick start button")
	flag.IntVar(&config.JoyButtonSelect, "joy-select", 10, "joystick select button")
	flag.IntVar(&config.JoyAxisX, "joy-x", 0, "joystick x-axis (for d-pad)")
	flag.IntVar(&config.JoyAxisY, "joy-y", 1, "joystick y-axis (for d-pad)")
}
