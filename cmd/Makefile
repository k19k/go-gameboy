include $(GOROOT)/src/Make.inc

TARG=../go-gameboy
GOFILES=\
	main.go
PREREQ+=../pkg/_obj/gameboy.a

GC+=-I../pkg/_obj
LD+=-L../pkg/_obj

include $(GOROOT)/src/Make.cmd
