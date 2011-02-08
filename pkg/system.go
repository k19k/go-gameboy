// Copyright 2011 Kevin Bulusek. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gameboy

import (
	"fmt"
	"io"
	"⚛sdl"
	"os"
)

type Config struct {
	SaveDir      string
	Verbose      bool
	Debug        bool
	Scale        int
	AudioFreq    int
	AudioBuffers int
	Fullscreen   bool

	Joystick        int
	JoyButtonA      int
	JoyButtonB      int
	JoyButtonStart  int
	JoyButtonSelect int
	JoyAxisX        int
	JoyAxisY        int
}

func Start(path string, cfg Config, in <-chan int, out chan<- interface{}) {
	var rom romImage
	var err interface{}

	defer func() {
		out <- err
	}()

	if rom, err = loadROM(path); err != nil {
		return
	}

	if cfg.Verbose {
		rom.printInfo()
	}

	if sdl.Init(sdl.INIT_VIDEO|sdl.INIT_AUDIO|sdl.INIT_JOYSTICK) != 0 {
		err = sdl.GetError()
		return
	}
	defer sdl.Quit()

	var mem *memory
	if mem, err = newMemory(rom, &cfg); err != nil {
		return
	}

	if e := mem.load(cfg.SaveDir); e != nil && cfg.Verbose {
		fmt.Fprintf(os.Stderr, "load failed: %v\n", e)
	}

	var audio *mixer
	if audio, err = newMixer(mem); err != nil {
		return
	}
	defer audio.close()

	sys := newCPU(mem)
	lcd := newDisplay(mem)

	mem.connect(sys, lcd, audio)

	joy := openJoystick(&cfg)
	if joy != nil {
		defer joy.Close()
	}

	go mem.monitorEvents()

	run(&cfg, sys, in)

	if e := mem.save(cfg.SaveDir); e != nil && cfg.Verbose {
		fmt.Fprintf(os.Stderr, "save failed: %v\n", e)
	}
}

func run(cfg *Config, sys *cpu, in <-chan int) {
	defer func() {
		if e := recover(); e != nil {
			if cfg.Debug {
				fmt.Fprintf(os.Stderr, "panic: %v\n\n", e)
				sys.dump(os.Stderr)
				fmt.Fprint(os.Stderr, "RUNTIME TRACE\n\n")
			}
			panic(e)
		}
	}()

	t := 0
	for {
		if t >= refreshTicks {
			if _, ok := <-in; ok || sys.quit {
				return
			}
			t = 0
		}
		s := 0
		for s < 10 {
			ts := sys.step()
			sys.updateTimers(ts)
			s += ts
		}
		sys.lcd.step(s)
		sys.audio.step(s)
		t += s
	}
}

func openJoystick(cfg *Config) (joy *sdl.Joystick) {
	n := sdl.NumJoysticks()
	if n == 0 {
		if cfg.Verbose {
			fmt.Println("no joysticks")
		}
		return
	}

	if cfg.Joystick >= n {
		if cfg.Verbose {
			fmt.Printf("no such joystick: %d (found %d)\n",
				cfg.Joystick, n)
		}
		return
	}

	joy = sdl.JoystickOpen(cfg.Joystick)
	if joy == nil {
		fmt.Fprintf(os.Stderr,
			"failed to open joystick: %v\n",
			sdl.GetError())
		return
	}

	sdl.JoystickEventState(sdl.ENABLE)

	if cfg.Verbose {
		fmt.Printf("using joystick %d\n", cfg.Joystick)
	}

	return joy
}

func (rom romImage) printInfo() {
	fmt.Printf("Loaded ROM image '%s'\n", rom.title())
	fmt.Printf("Logo match: %t\n", rom.checkLogo())
	fmt.Printf("Header checksum: %t\n", rom.doHeaderChecksum())
	fmt.Printf("Global checksum: %t\n", rom.doGlobalChecksum())
	fmt.Printf("ERAM: %d bytes\n", rom.ramSize())
	if mbc, err := rom.mbcType(); err == nil {
		fmt.Printf("MBC: %d\n", mbc)
	}
}

func (sys *cpu) dump(w io.Writer) {
	fmt.Fprintf(w,
		"LAST INSTRUCTION\n"+
			"%04X\t%s\n\n"+
			"CPU STATE\n"+
			"%v\n\n",
		sys.mar, sys.disasm(sys.mar), sys)
	sys.dumpStack(w)
	sys.memory.dump(w)
}
