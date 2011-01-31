include $(GOROOT)/src/Make.inc

TARG=gameboy
GOFILES=\
	mem.go\
	cpu.go\
	gpu.go\
	main.go

include $(GOROOT)/src/Make.cmd
