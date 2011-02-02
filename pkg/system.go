package gameboy

import (
	"fmt"
	"io"
	"sdl"
	"os"
	"os/signal"
)

type Config struct {
	SaveDir string
	Verbose bool
	Debug   bool
}

func Start(path string, cfg Config, quit chan int) (err interface{}) {
	var rom romImage
	rom, err = loadROM(path)
	if err != nil {
		return
	}

	if cfg.Verbose {
		rom.printInfo()
	}

	if sdl.Init(sdl.INIT_VIDEO) != 0 {
		err = sdl.GetError()
		return
	}

	var mem *memory
	mem, err = newMemory(rom)
	if err != nil {
		return
	}

	if e := mem.load(cfg.SaveDir); e != nil {
		fmt.Fprintf(os.Stderr, "load failed: %v\n", e)
	}

	sys := newCPU(mem)
	lcd := newDisplay(mem)

	go func() {
		run(&cfg, sys, lcd)
		if e := mem.save(cfg.SaveDir); e != nil {
			fmt.Fprintf(os.Stderr, "save failed: %v\n", e)
		}
		quit <- 1
	}()
	return
}

func run(cfg *Config, sys *cpu, lcd *display) {
	defer sdl.Quit()
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
		if sig, ok := <-signal.Incoming; ok {
			if cfg.Verbose {
				fmt.Printf("\nReceived %v, cleaning up\n", sig)
			}
			break
		}
		var s int
		for s = 0; s < 10; s += sys.step() {
		}
		lcd.step(s)
		t += s
	}
	if cfg.Debug {
		fmt.Printf("total ticks: %d\n", t)
		fmt.Printf("%v\n", sys)
	}
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
