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

	mem := newMemory(rom)
	sys := newCPU(mem)
	scr := newLCD(mem)

	ch = make(chan int)
	go run(ch, sys, scr)
	return ch, nil
}

func run(quit chan int, sys *cpu, scr *lcd) {
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
			fmt.Printf("\rReceived %v, cleaning up\n", sig)
			break
		}
		var s int
		for s = 0; s < 10; s += sys.step() {}
		scr.step(s)
		t += s
	}
	fmt.Printf("total ticks: %d\n", t)
	fmt.Printf("%v\n", sys)

	quit <- 0
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
