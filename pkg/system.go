package gameboy

import (
	"fmt"
	"io"
	"io/ioutil"
	"sdl"
	"os"
	"os/signal"
)

func Start(path string) (ch chan int, error interface{}) {
	rom, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		return nil, err
	}

	if sdl.Init(sdl.INIT_VIDEO) != 0 {
		return nil, sdl.GetError()
	}

	mbc := NewMBC(rom)
	cpu := NewCPU(mbc)
	gpu := NewGPU(mbc)

	ch = make(chan int)
	go run(ch, mbc, cpu, gpu)
	return ch, nil
}

func run(quit chan int, mbc *MBC, cpu *CPU, gpu *GPU) {
	defer sdl.Quit()
	defer func() {
		if e := recover(); e != nil {
			fmt.Fprintf(os.Stderr, "panic: %v\n\n", e)
			Dump(os.Stderr, mbc, cpu)
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

	quit <- 0
}

func Dump(w io.Writer, mbc *MBC, cpu *CPU) {
	fmt.Fprintf(w,
		"LAST INSTRUCTION\n" +
		"%04X\t%s\n\n" +
		"CPU STATE\n" +
		"%v\n\n",
		cpu.PC, mbc.Disasm(cpu.PC), cpu)
	cpu.DumpStack(w)
	mbc.Dump(w)
}
