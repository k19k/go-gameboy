package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"sdl"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage: %s rom\n", os.Args[0])
		return
	}

	rom, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err);
		return
	}

	if sdl.Init(sdl.INIT_VIDEO) != 0 {
		fmt.Fprintf(os.Stderr, "%v\n", sdl.GetError())
		return
	}
	defer sdl.Quit()

	mbc := NewMBC(rom)
	cpu := NewCPU(mbc)
	gpu := NewGPU(mbc)
	run(mbc, cpu, gpu)
}

func run(mbc *MBC, cpu *CPU, gpu *GPU) {
	defer func() {
		if e := recover(); e != nil {
			fmt.Fprintf(os.Stderr,
				"panic: %v\n\n" +
				"LAST INSTRUCTION\n" +
				"%04X\t%s\n\n" +
				"CPU STATE\n" +
				"%v\n\n",
				e, cpu.PC, mbc.Disasm(cpu.PC), cpu)
			mbc.Dump(os.Stderr)
			fmt.Fprint(os.Stderr, "RUNTIME TRACE\n\n")
			panic(e)
		}
	}()
	t := 0
	for {
		if sig, ok := <-signal.Incoming; ok {
			fmt.Printf("\rReceived %v, cleaning up\n", sig)
			break
		}
		var s int
		for s = 0; s < 10; s += cpu.Step() {}
		gpu.Step(s)
		t += s
	}
	fmt.Printf("total ticks: %d\n", t)
	fmt.Printf("%v\n%v\n", cpu, cpu.mmu)
}
