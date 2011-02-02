package gameboy

import (
	"fmt"
	"io"
	"sdl"
	"os"
	"os/signal"
)

func Start(quit chan int, path string) (err interface{}) {
	var rom romImage
	rom, err = loadROM(path)
	if err != nil { return }

	fmt.Printf("Loaded ROM image '%s'\n", rom.title())
	fmt.Printf("Logo match: %t\n", rom.checkLogo())
	fmt.Printf("Header checksum: %t\n", rom.headerChecksum())
	fmt.Printf("Global checksum: %t\n", rom.globalChecksum())

	if mbc, e := rom.mbcType(); e == nil {
		fmt.Printf("MBC: %d\n", mbc)
	}

	if sdl.Init(sdl.INIT_VIDEO) != 0 {
		err = sdl.GetError()
		return
	}

	var mem *memory
	mem, err = newMemory(rom)
	if err != nil { return }

	sys := newCPU(mem)
	lcd := newDisplay(mem)

	go func() {
		run(sys, lcd)
		quit <- 1
	}()
	return
}

func run(sys *cpu, lcd *display) {
	defer sdl.Quit()
	defer func() {
		if e := recover(); e != nil {
			fmt.Fprintf(os.Stderr, "panic: %v\n\n", e)
			sys.dump(os.Stderr)
			fmt.Fprint(os.Stderr, "RUNTIME TRACE\n\n")
			panic(e)
		}
	}()

	t := 0
	for {
		if sig, ok := <-signal.Incoming; ok {
			fmt.Printf("\nReceived %v, cleaning up\n", sig)
			break
		}
		var s int
		for s = 0; s < 10; s += sys.step() {}
		lcd.step(s)
		t += s
	}
	fmt.Printf("total ticks: %d\n", t)
	fmt.Printf("%v\n", sys)
}

func (sys *cpu) dump(w io.Writer) {
	fmt.Fprintf(w,
		"LAST INSTRUCTION\n" +
		"%04X\t%s\n\n" +
		"CPU STATE\n" +
		"%v\n\n",
		sys.mar, sys.disasm(sys.mar), sys)
	sys.dumpStack(w)
	sys.memory.dump(w)
}
