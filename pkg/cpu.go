// Copyright 2011 Kevin Bulusek. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gameboy

import (
	"fmt"
	"io"
)

type cpu struct {
	*memory
	a, b, c, d, e    byte
	hl, pc, sp       uint16
	fz, fn, fh, fc   bool
	ime, halt, pause bool
	mar              uint16
	stack            uint16
}

var fdxTable, fdxTableCB [256]func(*cpu) int

func newCPU(m *memory) *cpu {
	return &cpu{memory: m,
		a: 0x01, b: 0x00, c: 0x13, d: 0x00, e: 0xD8,
		hl: 0x014D, pc: 0x0100, sp: 0xFFFE,
		fz: true, fn: false, fh: true, fc: true,
		ime: true, halt: false, pause: true,
		mar: 0x0100, stack: 0xFFFE}
}

func (sys *cpu) String() string {
	return fmt.Sprintf(
		"<cpu AF=%04X BC=%04X DE=%04X HL=%04X\n"+
			"     PC=%04X SP=%04X\n"+
			"     IME=%t Halt=%t Pause=%t>",
		sys.af(), sys.bc(), sys.de(), sys.hl,
		sys.pc, sys.sp, sys.ime, sys.halt, sys.pause)
}

func (sys *cpu) step() int {
	if sys.ime {
		f := sys.readPort(portIF)
		e := sys.readPort(portIE)
		mask := f & e & 0x1F
		if mask != 0 {
			return sys.irq(mask, f)
		}
	}
	if !sys.halt {
		if sys.pc >= 0x8000 && sys.pc < 0xFF80 &&
			sys.pc < 0xC000 && sys.pc >= 0xFE00 {
			panic("executing data")
		}
		sys.mar = sys.pc
		//fmt.Printf("%04X %s\n", sys.pc, sys.disasm(sys.pc))
		return sys.fdx()
	}
	return 4
}

func (sys *cpu) irq(mask, f byte) int {
	sys.ime = false
	sys.halt = false
	sys.push(sys.pc)
	if mask&0x01 != 0 {
		sys.pc = vblankAddr
		f &^= 0x01
	} else if mask&0x02 != 0 {
		sys.pc = lcdStatusAddr
		f &^= 0x02
	} else if mask&0x04 != 0 {
		sys.pc = timerAddr
		f &^= 0x04
	} else if mask&0x08 != 0 {
		sys.pc = serialAddr
		f &^= 0x08
	} else if mask&0x10 != 0 {
		sys.pc = joypadAddr
		f &^= 0x10
	}
	sys.writePort(portIF, f)
	return 8
}

func (sys *cpu) dumpStack(w io.Writer) {
	addr := sys.stack - 2
	read := func() (x uint16, e interface{}) {
		defer func() {
			addr -= 2
			e = recover()
		}()
		return sys.readWord(addr), e
	}
	fmt.Fprintf(w, "STACK ┬  %04X\n", sys.stack)
	if sys.stack == sys.sp {
		fmt.Fprintln(w, "      ┴  (empty)")
	}
	for addr >= sys.sp {
		if addr == sys.sp {
			fmt.Fprintf(w, "   SP ╰→ ")
		} else {
			fmt.Fprintf(w, "      │  ")
		}
		fmt.Fprintf(w, "%04X ", addr)
		x, e := read()
		if e != nil {
			fmt.Fprintln(w, "(read failed!)")
		} else {
			fmt.Fprintf(w, "%04Xh\n", x)
		}
	}
	fmt.Fprintln(w)
}

// Fetch-decode-execute. Returns cycles/4. One cycle is 1/4194304
// seconds.
func (sys *cpu) fdx() int {
	return fdxTable[sys.fetchByte()](sys)
}

func (sys *cpu) jr(pred bool) int {
	x := sys.fetchByte()
	if pred {
		sys.pc += uint16(int8(x))
		return 3
	}
	return 2
}

func (sys *cpu) ret(pred bool) int {
	if pred {
		sys.pc = sys.pop()
		return 5
	}
	return 2
}

func (sys *cpu) jp(pred bool) int {
	x := sys.fetchWord()
	if pred {
		sys.pc = x
		return 4
	}
	return 3
}

func (sys *cpu) call(pred bool) int {
	x := sys.fetchWord()
	if pred {
		sys.push(sys.pc)
		sys.pc = x
		return 6
	}
	return 3
}

func (sys *cpu) rst(addr uint16) int {
	sys.push(sys.pc)
	sys.pc = addr
	return 8
}

func (sys *cpu) push(x uint16) {
	sys.sp -= 2
	sys.writeWord(sys.sp, x)
	//fmt.Printf("-> SP=%04Xh *=%04Xh\n", sys.sp, x)
}

func (sys *cpu) pop() uint16 {
	x := sys.readWord(sys.sp)
	//fmt.Printf("<- SP=%04Xh *=%04Xh\n", sys.sp, x)
	sys.sp += 2
	return x
}

func (sys *cpu) inc(x byte) byte {
	y := x + 1
	sys.fz = y == 0
	sys.fn = false
	sys.fh = (y & 0x0F) == 0
	return y
}

func (sys *cpu) dec(x byte) byte {
	y := x - 1
	sys.fz = y == 0
	sys.fn = true
	sys.fh = (y & 0x0F) == 0x0F
	return y
}

func (sys *cpu) add16(x, y uint16) uint16 {
	x1 := x + y
	sys.fn = false
	sys.fh = x1&0x0FFF < x&0x0FFF
	sys.fc = x1 < x
	return x1
}

func (sys *cpu) add(x byte) {
	y := sys.a + x
	sys.fz = y == 0
	sys.fn = false
	sys.fh = y&0x0F < sys.a&0x0F
	sys.fc = y < sys.a
	sys.a = y
}

func (sys *cpu) adc(x byte) {
	fc := byte(0)
	if sys.fc {
		fc = 1
	}
	y := int(sys.a) + int(x) + int(fc)
	sys.fz = byte(y) == 0
	sys.fn = false
	sys.fh = sys.a&0x0F+x&0x0F+fc > 0x0F
	sys.fc = y > 0xFF
	sys.a = byte(y)
}

func (sys *cpu) sub(x byte) {
	y := sys.a - x
	sys.fz = y == 0
	sys.fn = true
	sys.fh = y&0x0F > sys.a&0x0F
	sys.fc = y > sys.a
	sys.a = y
}

func (sys *cpu) sbc(x byte) {
	fc := byte(0)
	if sys.fc {
		fc = 1
	}
	y := int(sys.a) - int(x) - int(fc)
	sys.fz = y == 0
	sys.fn = true
	sys.fh = sys.a&0x0F < x&0x0F+fc
	sys.fc = y < 0
	sys.a = byte(y)
}

func (sys *cpu) and(x byte) {
	sys.a &= x
	sys.fz = sys.a == 0
	sys.fn = false
	sys.fh = true
	sys.fc = false
}

func (sys *cpu) xor(x byte) {
	sys.a ^= x
	sys.fz = sys.a == 0
	sys.fn = false
	sys.fh = false
	sys.fc = false
}

func (sys *cpu) or(x byte) {
	sys.a |= x
	sys.fz = sys.a == 0
	sys.fn = false
	sys.fh = false
	sys.fc = false
}

func (sys *cpu) cp(x byte) {
	y := sys.a - x
	sys.fz = y == 0
	sys.fn = true
	sys.fh = y&0x0F > sys.a&0x0F
	sys.fc = y > sys.a
}

func (sys *cpu) rlc(x byte) byte {
	fc := x >> 7
	y := x<<1 | fc
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = fc == 1
	return y
}

func (sys *cpu) rrc(x byte) byte {
	fc := x << 7
	y := x>>1 | fc
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = fc == 0x80
	return y
}

func (sys *cpu) rl(x byte) byte {
	fc := byte(0)
	if sys.fc {
		fc = 1
	}
	y := x<<1 | fc
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = x&0x80 == 0x80
	return y
}

func (sys *cpu) rr(x byte) byte {
	fc := byte(0)
	if sys.fc {
		fc = 0x80
	}
	y := x>>1 | fc
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = x&0x01 == 0x01
	return y
}

func (sys *cpu) sla(x byte) byte {
	y := x << 1
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = x&0x80 == 0x80
	return y
}

func (sys *cpu) sra(x byte) byte {
	y := byte(int8(x) >> 1)
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = x&1 == 1
	return y
}

func (sys *cpu) srl(x byte) byte {
	y := x >> 1
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = x&1 == 1
	return y
}

func (sys *cpu) swap(x byte) byte {
	y := x>>4 | x<<4
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = false
	return y
}

func (sys *cpu) bit(n, x byte) {
	sys.fz = x&(1<<n) == 0
	sys.fn = false
	sys.fh = true
}

func res(n, x byte) byte {
	return x &^ (1 << n)
}

func set(n, x byte) byte {
	return x | 1<<n
}

// DAA and DAS implementations based on pseudocode from 80386
// instruction set references.

func (sys *cpu) daa() {
	a := int(sys.a)
	if a&0x0F > 9 || sys.fh {
		a += 6
		sys.fh = true
	} else {
		sys.fh = false
	}
	if a > 0x9F || sys.fc {
		a += 0x60
		sys.fc = true
	} else {
		sys.fc = false
	}
	sys.a = byte(a)
	sys.fz = sys.a == 0
}

func (sys *cpu) das() {
	if sys.a&0x0F > 9 || sys.fh {
		sys.a -= 6
		sys.fh = true
	} else {
		sys.fh = false
	}
	if sys.a > 0x9F || sys.fc {
		sys.a -= 0x60
		sys.fc = true
	} else {
		sys.fc = false
	}
	sys.fz = sys.a == 0
}

func (sys *cpu) af() uint16 {
	a := uint16(sys.a)
	f := uint16(0)
	if sys.fz {
		f |= 0x80
	}
	if sys.fn {
		f |= 0x40
	}
	if sys.fh {
		f |= 0x20
	}
	if sys.fc {
		f |= 0x10
	}
	return (a << 8) | f
}

func (sys *cpu) waf(x uint16) {
	sys.a = byte(x >> 8)
	sys.fz = (x & 0x80) == 0x80
	sys.fn = (x & 0x40) == 0x40
	sys.fh = (x & 0x20) == 0x20
	sys.fc = (x & 0x10) == 0x10
}

func (sys *cpu) bc() uint16 {
	b := uint16(sys.b)
	c := uint16(sys.c)
	return (b << 8) | c
}

func (sys *cpu) wbc(x uint16) {
	sys.b = byte(x >> 8)
	sys.c = byte(x)
}

func (sys *cpu) de() uint16 {
	d := uint16(sys.d)
	e := uint16(sys.e)
	return (d << 8) | e
}

func (sys *cpu) wde(x uint16) {
	sys.d = byte(x >> 8)
	sys.e = byte(x)
}

func (sys *cpu) h() byte {
	return byte(sys.hl >> 8)
}

func (sys *cpu) wh(x byte) {
	sys.hl = (uint16(x) << 8) | (sys.hl & 0xFF)
}

func (sys *cpu) l() byte {
	return byte(sys.hl)
}

func (sys *cpu) wl(x byte) {
	sys.hl = uint16(x) | (sys.hl & 0xFF00)
}

func (sys *cpu) fetchByte() byte {
	pc := sys.pc
	sys.pc++
	return sys.readByte(pc)
}

func (sys *cpu) fetchWord() uint16 {
	pc := sys.pc
	sys.pc += 2
	return sys.readWord(pc)
}

func init() {
	fdxTable = [256]func(*cpu) int{
		0x00: func(sys *cpu) int { // NOP
			return 1
		},
		0x01: func(sys *cpu) int { // LD BC,d16
			sys.wbc(sys.fetchWord())
			return 3
		},
		0x02: func(sys *cpu) int { // LD (BC),A
			sys.writeByte(sys.bc(), sys.a)
			return 2
		},
		0x03: func(sys *cpu) int { // INC BC
			sys.wbc(sys.bc() + 1)
			return 2
		},
		0x04: func(sys *cpu) int { // INC B
			sys.b = sys.inc(sys.b)
			return 1
		},
		0x05: func(sys *cpu) int { // DEC B
			sys.b = sys.dec(sys.b)
			return 1
		},
		0x06: func(sys *cpu) int { // LD B,d8
			sys.b = sys.fetchByte()
			return 2
		},
		0x07: func(sys *cpu) int { // RLCA
			sys.a = sys.rlc(sys.a)
			return 1
		},
		0x08: func(sys *cpu) int { // LD (a16),SP
			sys.writeWord(sys.fetchWord(), sys.sp)
			return 5
		},
		0x09: func(sys *cpu) int { // ADD HL,BC
			sys.hl = sys.add16(sys.hl, sys.bc())
			return 2
		},
		0x0A: func(sys *cpu) int { // LD A,(BC)
			sys.a = sys.readByte(sys.bc())
			return 2
		},
		0x0B: func(sys *cpu) int { // DEC BC
			sys.wbc(sys.bc() - 1)
			return 2
		},
		0x0C: func(sys *cpu) int { // INC C
			sys.c = sys.inc(sys.c)
			return 1
		},
		0x0D: func(sys *cpu) int { // DEC C
			sys.c = sys.dec(sys.c)
			return 1
		},
		0x0E: func(sys *cpu) int { // LD C,d8
			sys.c = sys.fetchByte()
			return 2
		},
		0x0F: func(sys *cpu) int { // RRCA
			sys.a = sys.rrc(sys.a)
			return 1
		},

		0x10: func(sys *cpu) int { // STOP
			sys.pause = true
			return 1
		},
		0x11: func(sys *cpu) int { // LD DE,d16
			sys.wde(sys.fetchWord())
			return 3
		},
		0x12: func(sys *cpu) int { // LD (DE),A
			sys.writeByte(sys.de(), sys.a)
			return 2
		},
		0x13: func(sys *cpu) int { // INC DE
			sys.wde(sys.de() + 1)
			return 2
		},
		0x14: func(sys *cpu) int { // INC D
			sys.d = sys.inc(sys.d)
			return 1
		},
		0x15: func(sys *cpu) int { // DEC D
			sys.d = sys.dec(sys.d)
			return 1
		},
		0x16: func(sys *cpu) int { // LD D,d8
			sys.d = sys.fetchByte()
			return 2
		},
		0x17: func(sys *cpu) int { // RLA
			sys.a = sys.rl(sys.a)
			return 1
		},
		0x18: func(sys *cpu) int { // JR r8
			return sys.jr(true)
		},
		0x19: func(sys *cpu) int { // ADD HL,DE
			sys.hl = sys.add16(sys.hl, sys.de())
			return 2
		},
		0x1A: func(sys *cpu) int { // LD A,(DE)
			sys.a = sys.readByte(sys.de())
			return 2
		},
		0x1B: func(sys *cpu) int { // DEC DE
			sys.wde(sys.de() - 1)
			return 2
		},
		0x1C: func(sys *cpu) int { // INC E
			sys.e = sys.inc(sys.e)
			return 1
		},
		0x1D: func(sys *cpu) int { // DEC E
			sys.e = sys.dec(sys.e)
			return 1
		},
		0x1E: func(sys *cpu) int { // LD E,d8
			sys.e = sys.fetchByte()
			return 2
		},
		0x1F: func(sys *cpu) int { // RRA
			sys.a = sys.rr(sys.a)
			return 1
		},

		0x20: func(sys *cpu) int { // JR NZ,r8
			return sys.jr(!sys.fz)
		},
		0x21: func(sys *cpu) int { // LD HL,d16
			sys.hl = sys.fetchWord()
			return 3
		},
		0x22: func(sys *cpu) int { // LD (HL+),A
			sys.writeByte(sys.hl, sys.a)
			sys.hl++
			return 2
		},
		0x23: func(sys *cpu) int { // INC HL
			sys.hl++
			return 2
		},
		0x24: func(sys *cpu) int { // INC H
			sys.wh(sys.inc(sys.h()))
			return 1
		},
		0x25: func(sys *cpu) int { // DEC H
			sys.wh(sys.dec(sys.h()))
			return 1
		},
		0x26: func(sys *cpu) int { // LD H,d8
			sys.wh(sys.fetchByte())
			return 2
		},
		0x27: func(sys *cpu) int { // DAA
			if sys.fn {
				sys.das()
			} else {
				sys.daa()
			}
			return 1
		},
		0x28: func(sys *cpu) int { // JR Z,r8
			return sys.jr(sys.fz)
		},
		0x29: func(sys *cpu) int { // ADD HL,HL
			sys.hl = sys.add16(sys.hl, sys.hl)
			return 2
		},
		0x2A: func(sys *cpu) int { // LD A,(HL+)
			sys.a = sys.readByte(sys.hl)
			sys.hl++
			return 2
		},
		0x2B: func(sys *cpu) int { // DEC HL
			sys.hl--
			return 2
		},
		0x2C: func(sys *cpu) int { // INC L
			sys.wl(sys.inc(sys.l()))
			return 1
		},
		0x2D: func(sys *cpu) int { // DEC L
			sys.wl(sys.dec(sys.l()))
			return 1
		},
		0x2E: func(sys *cpu) int { // LD L,d8
			sys.wl(sys.fetchByte())
			return 2
		},
		0x2F: func(sys *cpu) int { // CPL
			sys.a ^= 0xFF
			sys.fn = true
			sys.fh = true
			return 1
		},

		0x30: func(sys *cpu) int { // JR NC,r8
			return sys.jr(!sys.fc)
		},
		0x31: func(sys *cpu) int { // LD SP,d16
			sys.sp = sys.fetchWord()
			sys.stack = sys.sp
			return 3
		},
		0x32: func(sys *cpu) int { // LD (HL-),A
			sys.writeByte(sys.hl, sys.a)
			sys.hl--
			return 2
		},
		0x33: func(sys *cpu) int { // INC SP
			sys.sp++
			return 2
		},
		0x34: func(sys *cpu) int { // INC (HL)
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, sys.inc(x))
			return 3
		},
		0x35: func(sys *cpu) int { // DEC (HL)
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, sys.dec(x))
			return 3
		},
		0x36: func(sys *cpu) int { // LD (HL),d8
			sys.writeByte(sys.hl, sys.fetchByte())
			return 3
		},
		0x37: func(sys *cpu) int { // SCF
			sys.fn = false
			sys.fh = false
			sys.fc = true
			return 1
		},
		0x38: func(sys *cpu) int { // JR C,r8
			return sys.jr(sys.fc)
		},
		0x39: func(sys *cpu) int { // ADD HL,SP
			sys.hl = sys.add16(sys.hl, sys.sp)
			return 2
		},
		0x3A: func(sys *cpu) int { // LD A,(HL-)
			sys.a = sys.readByte(sys.hl)
			sys.hl--
			return 2
		},
		0x3B: func(sys *cpu) int { // DEC SP
			sys.sp--
			return 2
		},
		0x3C: func(sys *cpu) int { // INC A
			sys.a = sys.inc(sys.a)
			return 1
		},
		0x3D: func(sys *cpu) int { // DEC A
			sys.a = sys.dec(sys.a)
			return 1
		},
		0x3E: func(sys *cpu) int { // LD A,d8
			sys.a = sys.fetchByte()
			return 2
		},
		0x3F: func(sys *cpu) int { // CCF
			sys.fn = false
			sys.fh = false
			sys.fc = !sys.fc
			return 1
		},

		// LD Instructions ///////////////////////////////////

		0x40: func(sys *cpu) int { // LD B,r
			return 1
		},
		0x41: func(sys *cpu) int {
			sys.b = sys.c
			return 1
		},
		0x42: func(sys *cpu) int {
			sys.b = sys.d
			return 1
		},
		0x43: func(sys *cpu) int {
			sys.b = sys.e
			return 1
		},
		0x44: func(sys *cpu) int {
			sys.b = sys.h()
			return 1
		},
		0x45: func(sys *cpu) int {
			sys.b = sys.l()
			return 1
		},
		0x46: func(sys *cpu) int {
			sys.b = sys.readByte(sys.hl)
			return 2
		},
		0x47: func(sys *cpu) int {
			sys.b = sys.a
			return 1
		},

		0x48: func(sys *cpu) int { // LD C,r
			sys.c = sys.b
			return 1
		},
		0x49: func(sys *cpu) int {
			return 1
		},
		0x4A: func(sys *cpu) int {
			sys.c = sys.d
			return 1
		},
		0x4B: func(sys *cpu) int {
			sys.c = sys.e
			return 1
		},
		0x4C: func(sys *cpu) int {
			sys.c = sys.h()
			return 1
		},
		0x4D: func(sys *cpu) int {
			sys.c = sys.l()
			return 1
		},
		0x4E: func(sys *cpu) int {
			sys.c = sys.readByte(sys.hl)
			return 2
		},
		0x4F: func(sys *cpu) int {
			sys.c = sys.a
			return 1
		},

		0x50: func(sys *cpu) int { // LD D,r
			sys.d = sys.b
			return 1
		},
		0x51: func(sys *cpu) int {
			sys.d = sys.c
			return 1
		},
		0x52: func(sys *cpu) int {
			return 1
		},
		0x53: func(sys *cpu) int {
			sys.d = sys.e
			return 1
		},
		0x54: func(sys *cpu) int {
			sys.d = sys.h()
			return 1
		},
		0x55: func(sys *cpu) int {
			sys.d = sys.l()
			return 1
		},
		0x56: func(sys *cpu) int {
			sys.d = sys.readByte(sys.hl)
			return 2
		},
		0x57: func(sys *cpu) int {
			sys.d = sys.a
			return 1
		},

		0x58: func(sys *cpu) int { // LD E,r
			sys.e = sys.b
			return 1
		},
		0x59: func(sys *cpu) int {
			sys.e = sys.c
			return 1
		},
		0x5A: func(sys *cpu) int {
			sys.e = sys.d
			return 1
		},
		0x5B: func(sys *cpu) int {
			return 1
		},
		0x5C: func(sys *cpu) int {
			sys.e = sys.h()
			return 1
		},
		0x5D: func(sys *cpu) int {
			sys.e = sys.l()
			return 1
		},
		0x5E: func(sys *cpu) int {
			sys.e = sys.readByte(sys.hl)
			return 2
		},
		0x5F: func(sys *cpu) int {
			sys.e = sys.a
			return 1
		},

		0x60: func(sys *cpu) int { // LD H,r
			sys.wh(sys.b)
			return 1
		},
		0x61: func(sys *cpu) int {
			sys.wh(sys.c)
			return 1
		},
		0x62: func(sys *cpu) int {
			sys.wh(sys.d)
			return 1
		},
		0x63: func(sys *cpu) int {
			sys.wh(sys.e)
			return 1
		},
		0x64: func(sys *cpu) int {
			return 1
		},
		0x65: func(sys *cpu) int {
			sys.wh(sys.l())
			return 1
		},
		0x66: func(sys *cpu) int {
			sys.wh(sys.readByte(sys.hl))
			return 2
		},
		0x67: func(sys *cpu) int {
			sys.wh(sys.a)
			return 1
		},

		0x68: func(sys *cpu) int { // LD L,r
			sys.wl(sys.b)
			return 1
		},
		0x69: func(sys *cpu) int {
			sys.wl(sys.c)
			return 1
		},
		0x6A: func(sys *cpu) int {
			sys.wl(sys.d)
			return 1
		},
		0x6B: func(sys *cpu) int {
			sys.wl(sys.e)
			return 1
		},
		0x6C: func(sys *cpu) int {
			sys.wl(sys.h())
			return 1
		},
		0x6D: func(sys *cpu) int {
			return 1
		},
		0x6E: func(sys *cpu) int {
			sys.wl(sys.readByte(sys.hl))
			return 2
		},
		0x6F: func(sys *cpu) int {
			sys.wl(sys.a)
			return 1
		},

		0x70: func(sys *cpu) int { // LD (HL),r
			sys.writeByte(sys.hl, sys.b)
			return 2
		},
		0x71: func(sys *cpu) int {
			sys.writeByte(sys.hl, sys.c)
			return 2
		},
		0x72: func(sys *cpu) int {
			sys.writeByte(sys.hl, sys.d)
			return 2
		},
		0x73: func(sys *cpu) int {
			sys.writeByte(sys.hl, sys.e)
			return 2
		},
		0x74: func(sys *cpu) int {
			sys.writeByte(sys.hl, sys.h())
			return 2
		},
		0x75: func(sys *cpu) int {
			sys.writeByte(sys.hl, sys.l())
			return 2
		},
		0x76: func(sys *cpu) int { // HALT
			sys.halt = true
			return 1
		},
		0x77: func(sys *cpu) int {
			sys.writeByte(sys.hl, sys.a)
			return 2
		},

		0x78: func(sys *cpu) int { // LD A,r
			sys.a = sys.b
			return 1
		},
		0x79: func(sys *cpu) int {
			sys.a = sys.c
			return 1
		},
		0x7A: func(sys *cpu) int {
			sys.a = sys.d
			return 1
		},
		0x7B: func(sys *cpu) int {
			sys.a = sys.e
			return 1
		},
		0x7C: func(sys *cpu) int {
			sys.a = sys.h()
			return 1
		},
		0x7D: func(sys *cpu) int {
			sys.a = sys.l()
			return 1
		},
		0x7E: func(sys *cpu) int {
			sys.a = sys.readByte(sys.hl)
			return 2
		},
		0x7F: func(sys *cpu) int {
			return 1
		},

		// Math Instructions /////////////////////////////////

		0x80: func(sys *cpu) int { // ADD A,r
			sys.add(sys.b)
			return 1
		},
		0x81: func(sys *cpu) int {
			sys.add(sys.c)
			return 1
		},
		0x82: func(sys *cpu) int {
			sys.add(sys.d)
			return 1
		},
		0x83: func(sys *cpu) int {
			sys.add(sys.e)
			return 1
		},
		0x84: func(sys *cpu) int {
			sys.add(sys.h())
			return 1
		},
		0x85: func(sys *cpu) int {
			sys.add(sys.l())
			return 1
		},
		0x86: func(sys *cpu) int {
			sys.add(sys.readByte(sys.hl))
			return 2
		},
		0x87: func(sys *cpu) int {
			sys.add(sys.a)
			return 1
		},

		0x88: func(sys *cpu) int { // ADC A,r
			sys.adc(sys.b)
			return 1
		},
		0x89: func(sys *cpu) int {
			sys.adc(sys.c)
			return 1
		},
		0x8A: func(sys *cpu) int {
			sys.adc(sys.d)
			return 1
		},
		0x8B: func(sys *cpu) int {
			sys.adc(sys.e)
			return 1
		},
		0x8C: func(sys *cpu) int {
			sys.adc(sys.h())
			return 1
		},
		0x8D: func(sys *cpu) int {
			sys.adc(sys.l())
			return 1
		},
		0x8E: func(sys *cpu) int {
			sys.adc(sys.readByte(sys.hl))
			return 2
		},
		0x8F: func(sys *cpu) int {
			sys.adc(sys.a)
			return 1
		},

		0x90: func(sys *cpu) int { // SUB r
			sys.sub(sys.b)
			return 1
		},
		0x91: func(sys *cpu) int {
			sys.sub(sys.c)
			return 1
		},
		0x92: func(sys *cpu) int {
			sys.sub(sys.d)
			return 1
		},
		0x93: func(sys *cpu) int {
			sys.sub(sys.e)
			return 1
		},
		0x94: func(sys *cpu) int {
			sys.sub(sys.h())
			return 1
		},
		0x95: func(sys *cpu) int {
			sys.sub(sys.l())
			return 1
		},
		0x96: func(sys *cpu) int {
			sys.sub(sys.readByte(sys.hl))
			return 2
		},
		0x97: func(sys *cpu) int {
			sys.sub(sys.a)
			return 1
		},

		0x98: func(sys *cpu) int { // SBC r
			sys.sbc(sys.b)
			return 1
		},
		0x99: func(sys *cpu) int {
			sys.sbc(sys.c)
			return 1
		},
		0x9A: func(sys *cpu) int {
			sys.sbc(sys.d)
			return 1
		},
		0x9B: func(sys *cpu) int {
			sys.sbc(sys.e)
			return 1
		},
		0x9C: func(sys *cpu) int {
			sys.sbc(sys.h())
			return 1
		},
		0x9D: func(sys *cpu) int {
			sys.sbc(sys.l())
			return 1
		},
		0x9E: func(sys *cpu) int {
			sys.sbc(sys.readByte(sys.hl))
			return 2
		},
		0x9F: func(sys *cpu) int {
			sys.sbc(sys.a)
			return 1
		},

		0xA0: func(sys *cpu) int { // AND r
			sys.and(sys.b)
			return 1
		},
		0xA1: func(sys *cpu) int {
			sys.and(sys.c)
			return 1
		},
		0xA2: func(sys *cpu) int {
			sys.and(sys.d)
			return 1
		},
		0xA3: func(sys *cpu) int {
			sys.and(sys.e)
			return 1
		},
		0xA4: func(sys *cpu) int {
			sys.and(sys.h())
			return 1
		},
		0xA5: func(sys *cpu) int {
			sys.and(sys.l())
			return 1
		},
		0xA6: func(sys *cpu) int {
			sys.and(sys.readByte(sys.hl))
			return 2
		},
		0xA7: func(sys *cpu) int {
			sys.and(sys.a)
			return 1
		},

		0xA8: func(sys *cpu) int { // XOR r
			sys.xor(sys.b)
			return 1
		},
		0xA9: func(sys *cpu) int {
			sys.xor(sys.c)
			return 1
		},
		0xAA: func(sys *cpu) int {
			sys.xor(sys.d)
			return 1
		},
		0xAB: func(sys *cpu) int {
			sys.xor(sys.e)
			return 1
		},
		0xAC: func(sys *cpu) int {
			sys.xor(sys.h())
			return 1
		},
		0xAD: func(sys *cpu) int {
			sys.xor(sys.l())
			return 1
		},
		0xAE: func(sys *cpu) int {
			sys.xor(sys.readByte(sys.hl))
			return 2
		},
		0xAF: func(sys *cpu) int {
			sys.xor(sys.a)
			return 1
		},

		0xB0: func(sys *cpu) int { // OR r
			sys.or(sys.b)
			return 1
		},
		0xB1: func(sys *cpu) int {
			sys.or(sys.c)
			return 1
		},
		0xB2: func(sys *cpu) int {
			sys.or(sys.d)
			return 1
		},
		0xB3: func(sys *cpu) int {
			sys.or(sys.e)
			return 1
		},
		0xB4: func(sys *cpu) int {
			sys.or(sys.h())
			return 1
		},
		0xB5: func(sys *cpu) int {
			sys.or(sys.l())
			return 1
		},
		0xB6: func(sys *cpu) int {
			sys.or(sys.readByte(sys.hl))
			return 2
		},
		0xB7: func(sys *cpu) int {
			sys.or(sys.a)
			return 1
		},

		0xB8: func(sys *cpu) int { // CP r
			sys.cp(sys.b)
			return 1
		},
		0xB9: func(sys *cpu) int {
			sys.cp(sys.c)
			return 1
		},
		0xBA: func(sys *cpu) int {
			sys.cp(sys.d)
			return 1
		},
		0xBB: func(sys *cpu) int {
			sys.cp(sys.e)
			return 1
		},
		0xBC: func(sys *cpu) int {
			sys.cp(sys.h())
			return 1
		},
		0xBD: func(sys *cpu) int {
			sys.cp(sys.l())
			return 1
		},
		0xBE: func(sys *cpu) int {
			sys.cp(sys.readByte(sys.hl))
			return 2
		},
		0xBF: func(sys *cpu) int {
			sys.cp(sys.a)
			return 1
		},

		// Misc Instructions /////////////////////////////////

		0xC0: func(sys *cpu) int { // RET NZ
			return sys.ret(!sys.fz)
		},
		0xC1: func(sys *cpu) int { // POP BC
			sys.wbc(sys.pop())
			return 3
		},
		0xC2: func(sys *cpu) int { // JP NZ,a16
			return sys.jp(!sys.fz)
		},
		0xC3: func(sys *cpu) int { // JP a16
			return sys.jp(true)
		},
		0xC4: func(sys *cpu) int { // CALL NZ,a16
			return sys.call(!sys.fz)
		},
		0xC5: func(sys *cpu) int { // PUSH BC
			sys.push(sys.bc())
			return 4
		},
		0xC6: func(sys *cpu) int { // ADD A,d8
			sys.add(sys.fetchByte())
			return 2
		},
		0xC7: func(sys *cpu) int { // RST 00H
			return sys.rst(0x00)
		},
		0xC8: func(sys *cpu) int { // RET Z
			return sys.ret(sys.fz)
		},
		0xC9: func(sys *cpu) int { // RET
			sys.pc = sys.pop()
			return 4
		},
		0xCA: func(sys *cpu) int { // JP Z,a16
			return sys.jp(sys.fz)
		},
		0xCB: func(sys *cpu) int { // ** PREFIX CB **
			return fdxTableCB[sys.fetchByte()](sys)
		},
		0xCC: func(sys *cpu) int { // CALL Z,a16
			return sys.call(sys.fz)
		},
		0xCD: func(sys *cpu) int { // CALL a16
			return sys.call(true)
		},
		0xCE: func(sys *cpu) int { // ADC A,d8
			sys.adc(sys.fetchByte())
			return 2
		},
		0xCF: func(sys *cpu) int { // RST 08H
			return sys.rst(0x08)
		},

		0xD0: func(sys *cpu) int { // RET NC
			return sys.ret(!sys.fc)
		},
		0xD1: func(sys *cpu) int { // POP DE
			sys.wde(sys.pop())
			return 3
		},
		0xD2: func(sys *cpu) int { // JP NC,a16
			return sys.jp(!sys.fc)
		},
		0xD3: func(sys *cpu) int {
			panic("cpu: invalid opcode 0xD3")
		},
		0xD4: func(sys *cpu) int { // CALL NC,a16
			return sys.call(!sys.fc)
		},
		0xD5: func(sys *cpu) int { // PUSH DE
			sys.push(sys.de())
			return 4
		},
		0xD6: func(sys *cpu) int { // SUB d8
			sys.sub(sys.fetchByte())
			return 2
		},
		0xD7: func(sys *cpu) int { // RST 10H
			return sys.rst(0x10)
		},
		0xD8: func(sys *cpu) int { // RET C
			return sys.ret(sys.fc)
		},
		0xD9: func(sys *cpu) int { // RETI
			sys.ime = true
			sys.pc = sys.pop()
			return 4
		},
		0xDA: func(sys *cpu) int { // JP C,a16
			return sys.jp(sys.fc)
		},
		0xDB: func(sys *cpu) int {
			panic("cpu: invalid opcode 0xDB")
		},
		0xDC: func(sys *cpu) int { // CALL C,a16
			return sys.call(sys.fc)
		},
		0xDD: func(sys *cpu) int {
			panic("cpu: invalid opcode 0xDD")
		},
		0xDE: func(sys *cpu) int { // SBC d8
			sys.sbc(sys.fetchByte())
			return 2
		},
		0xDF: func(sys *cpu) int { // RST 18H
			return sys.rst(0x18)
		},

		0xE0: func(sys *cpu) int { // LDH (a8),A
			addr := 0xFF00 + uint16(sys.fetchByte())
			sys.writePort(addr, sys.a)
			return 3
		},
		0xE1: func(sys *cpu) int { // POP HL
			sys.hl = sys.pop()
			return 3
		},
		0xE2: func(sys *cpu) int { // LD (C),A
			addr := 0xFF00 + uint16(sys.c)
			sys.writePort(addr, sys.a)
			return 2
		},
		0xE3: func(sys *cpu) int {
			panic("cpu: invalid opcode 0xE3")
		},
		0xE4: func(sys *cpu) int {
			panic("cpu: invalid opcode 0xE4")
		},
		0xE5: func(sys *cpu) int { // PUSH HL
			sys.push(sys.hl)
			return 4
		},
		0xE6: func(sys *cpu) int { // AND d8
			sys.and(sys.fetchByte())
			return 2
		},
		0xE7: func(sys *cpu) int { // RST 20H
			return sys.rst(0x20)
		},
		0xE8: func(sys *cpu) int { // ADD SP,r8
			x := sys.fetchByte()
			sys.sp = sys.add16(sys.sp, uint16(int8(x)))
			return 4
		},
		0xE9: func(sys *cpu) int { // JP (HL)
			sys.pc = sys.hl
			return 1
		},
		0xEA: func(sys *cpu) int { // LD (a16),A
			sys.writeByte(sys.fetchWord(), sys.a)
			return 4
		},
		0xEB: func(sys *cpu) int {
			panic("cpu: invalid opcode 0xEB")
		},
		0xEC: func(sys *cpu) int {
			panic("cpu: invalid opcode 0xEC")
		},
		0xED: func(sys *cpu) int {
			panic("cpu: invalid opcode 0xED")
		},
		0xEE: func(sys *cpu) int { // XOR d8
			sys.xor(sys.fetchByte())
			return 2
		},
		0xEF: func(sys *cpu) int { // RST 28H
			return sys.rst(0x28)
		},

		0xF0: func(sys *cpu) int { // LDH A,(a8)
			addr := 0xFF00 + uint16(sys.fetchByte())
			sys.a = sys.readPort(addr)
			return 3
		},
		0xF1: func(sys *cpu) int { // POP AF
			sys.waf(sys.pop())
			return 3
		},
		0xF2: func(sys *cpu) int { // LD A,(C)
			addr := 0xFF00 + uint16(sys.c)
			sys.a = sys.readPort(addr)
			return 2
		},
		0xF3: func(sys *cpu) int { // DI
			sys.ime = false
			return 1
		},
		0xF4: func(sys *cpu) int {
			panic("cpu: invalid opcode 0xF4")
		},
		0xF5: func(sys *cpu) int { // PUSH AF
			sys.push(sys.af())
			return 4
		},
		0xF6: func(sys *cpu) int { // OR d8
			sys.or(sys.fetchByte())
			return 2
		},
		0xF7: func(sys *cpu) int { // RST 30H
			return sys.rst(0x30)
		},
		0xF8: func(sys *cpu) int { // LD HL,SP+r8
			x := sys.fetchByte()
			sys.hl = sys.add16(sys.sp, uint16(int8(x)))
			return 3
		},
		0xF9: func(sys *cpu) int { // LD SP,HL
			sys.sp = sys.hl
			sys.stack = sys.sp
			return 2
		},
		0xFA: func(sys *cpu) int { // LD A,(a16)
			sys.a = sys.readByte(sys.fetchWord())
			return 4
		},
		0xFB: func(sys *cpu) int { // EI
			sys.ime = true
			return 1
		},
		0xFC: func(sys *cpu) int {
			panic("cpu: invalid opcode 0xFC")
		},
		0xFD: func(sys *cpu) int {
			panic("cpu: invalid opcode 0xFD")
		},
		0xFE: func(sys *cpu) int { // CP d8
			sys.cp(sys.fetchByte())
			return 2
		},
		0xFF: func(sys *cpu) int { // RST 38H
			return sys.rst(0x38)
		},
	}
	fdxTableCB = [256]func(*cpu) int{
		0x00: func(sys *cpu) int {
			sys.b = sys.rlc(sys.b)
			return 2
		},
		0x01: func(sys *cpu) int {
			sys.c = sys.rlc(sys.c)
			return 2
		},
		0x02: func(sys *cpu) int {
			sys.d = sys.rlc(sys.d)
			return 2
		},
		0x03: func(sys *cpu) int {
			sys.e = sys.rlc(sys.e)
			return 2
		},
		0x04: func(sys *cpu) int {
			sys.wh(sys.rlc(sys.h()))
			return 2
		},
		0x05: func(sys *cpu) int {
			sys.wl(sys.rlc(sys.l()))
			return 2
		},
		0x06: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, sys.rlc(x))
			return 4
		},
		0x07: func(sys *cpu) int {
			sys.a = sys.rlc(sys.a)
			return 2
		},

		0x08: func(sys *cpu) int {
			sys.b = sys.rrc(sys.b)
			return 2
		},
		0x09: func(sys *cpu) int {
			sys.c = sys.rrc(sys.c)
			return 2
		},
		0x0A: func(sys *cpu) int {
			sys.d = sys.rrc(sys.d)
			return 2
		},
		0x0B: func(sys *cpu) int {
			sys.e = sys.rrc(sys.e)
			return 2
		},
		0x0C: func(sys *cpu) int {
			sys.wh(sys.rrc(sys.h()))
			return 2
		},
		0x0D: func(sys *cpu) int {
			sys.wl(sys.rrc(sys.l()))
			return 2
		},
		0x0E: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, sys.rrc(x))
			return 4
		},
		0x0F: func(sys *cpu) int {
			sys.a = sys.rrc(sys.a)
			return 2
		},

		0x10: func(sys *cpu) int {
			sys.b = sys.rl(sys.b)
			return 2
		},
		0x11: func(sys *cpu) int {
			sys.c = sys.rl(sys.c)
			return 2
		},
		0x12: func(sys *cpu) int {
			sys.d = sys.rl(sys.d)
			return 2
		},
		0x13: func(sys *cpu) int {
			sys.e = sys.rl(sys.e)
			return 2
		},
		0x14: func(sys *cpu) int {
			sys.wh(sys.rl(sys.h()))
			return 2
		},
		0x15: func(sys *cpu) int {
			sys.wl(sys.rl(sys.l()))
			return 2
		},
		0x16: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, sys.rl(x))
			return 4
		},
		0x17: func(sys *cpu) int {
			sys.a = sys.rl(sys.a)
			return 2
		},

		0x18: func(sys *cpu) int {
			sys.b = sys.rr(sys.b)
			return 2
		},
		0x19: func(sys *cpu) int {
			sys.c = sys.rr(sys.c)
			return 2
		},
		0x1A: func(sys *cpu) int {
			sys.d = sys.rr(sys.d)
			return 2
		},
		0x1B: func(sys *cpu) int {
			sys.e = sys.rr(sys.e)
			return 2
		},
		0x1C: func(sys *cpu) int {
			sys.wh(sys.rr(sys.h()))
			return 2
		},
		0x1D: func(sys *cpu) int {
			sys.wl(sys.rr(sys.l()))
			return 2
		},
		0x1E: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, sys.rr(x))
			return 4
		},
		0x1F: func(sys *cpu) int {
			sys.a = sys.rr(sys.a)
			return 2
		},

		0x20: func(sys *cpu) int {
			sys.b = sys.sla(sys.b)
			return 2
		},
		0x21: func(sys *cpu) int {
			sys.c = sys.sla(sys.c)
			return 2
		},
		0x22: func(sys *cpu) int {
			sys.d = sys.sla(sys.d)
			return 2
		},
		0x23: func(sys *cpu) int {
			sys.e = sys.sla(sys.e)
			return 2
		},
		0x24: func(sys *cpu) int {
			sys.wh(sys.sla(sys.h()))
			return 2
		},
		0x25: func(sys *cpu) int {
			sys.wl(sys.sla(sys.l()))
			return 2
		},
		0x26: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, sys.sla(x))
			return 4
		},
		0x27: func(sys *cpu) int {
			sys.a = sys.sla(sys.a)
			return 2
		},

		0x28: func(sys *cpu) int {
			sys.b = sys.sra(sys.b)
			return 2
		},
		0x29: func(sys *cpu) int {
			sys.c = sys.sra(sys.c)
			return 2
		},
		0x2A: func(sys *cpu) int {
			sys.d = sys.sra(sys.d)
			return 2
		},
		0x2B: func(sys *cpu) int {
			sys.e = sys.sra(sys.e)
			return 2
		},
		0x2C: func(sys *cpu) int {
			sys.wh(sys.sra(sys.h()))
			return 2
		},
		0x2D: func(sys *cpu) int {
			sys.wl(sys.sra(sys.l()))
			return 2
		},
		0x2E: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, sys.sra(x))
			return 4
		},
		0x2F: func(sys *cpu) int {
			sys.a = sys.sra(sys.a)
			return 2
		},

		0x30: func(sys *cpu) int {
			sys.b = sys.swap(sys.b)
			return 2
		},
		0x31: func(sys *cpu) int {
			sys.c = sys.swap(sys.c)
			return 2
		},
		0x32: func(sys *cpu) int {
			sys.d = sys.swap(sys.d)
			return 2
		},
		0x33: func(sys *cpu) int {
			sys.e = sys.swap(sys.e)
			return 2
		},
		0x34: func(sys *cpu) int {
			sys.wh(sys.swap(sys.h()))
			return 2
		},
		0x35: func(sys *cpu) int {
			sys.wl(sys.swap(sys.l()))
			return 2
		},
		0x36: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, sys.swap(x))
			return 4
		},
		0x37: func(sys *cpu) int {
			sys.a = sys.swap(sys.a)
			return 2
		},

		0x38: func(sys *cpu) int {
			sys.b = sys.srl(sys.b)
			return 2
		},
		0x39: func(sys *cpu) int {
			sys.c = sys.srl(sys.c)
			return 2
		},
		0x3A: func(sys *cpu) int {
			sys.d = sys.srl(sys.d)
			return 2
		},
		0x3B: func(sys *cpu) int {
			sys.e = sys.srl(sys.e)
			return 2
		},
		0x3C: func(sys *cpu) int {
			sys.wh(sys.srl(sys.h()))
			return 2
		},
		0x3D: func(sys *cpu) int {
			sys.wl(sys.srl(sys.l()))
			return 2
		},
		0x3E: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, sys.srl(x))
			return 4
		},
		0x3F: func(sys *cpu) int {
			sys.a = sys.srl(sys.a)
			return 2
		},

		0x40: func(sys *cpu) int {
			sys.bit(0, sys.b)
			return 2
		},
		0x41: func(sys *cpu) int {
			sys.bit(0, sys.c)
			return 2
		},
		0x42: func(sys *cpu) int {
			sys.bit(0, sys.d)
			return 2
		},
		0x43: func(sys *cpu) int {
			sys.bit(0, sys.e)
			return 2
		},
		0x44: func(sys *cpu) int {
			sys.bit(0, sys.h())
			return 2
		},
		0x45: func(sys *cpu) int {
			sys.bit(0, sys.l())
			return 2
		},
		0x46: func(sys *cpu) int {
			sys.bit(0, sys.readByte(sys.hl))
			return 4
		},
		0x47: func(sys *cpu) int {
			sys.bit(0, sys.a)
			return 2
		},

		0x48: func(sys *cpu) int {
			sys.bit(1, sys.b)
			return 2
		},
		0x49: func(sys *cpu) int {
			sys.bit(1, sys.c)
			return 2
		},
		0x4A: func(sys *cpu) int {
			sys.bit(1, sys.d)
			return 2
		},
		0x4B: func(sys *cpu) int {
			sys.bit(1, sys.e)
			return 2
		},
		0x4C: func(sys *cpu) int {
			sys.bit(1, sys.h())
			return 2
		},
		0x4D: func(sys *cpu) int {
			sys.bit(1, sys.l())
			return 2
		},
		0x4E: func(sys *cpu) int {
			sys.bit(1, sys.readByte(sys.hl))
			return 4
		},
		0x4F: func(sys *cpu) int {
			sys.bit(1, sys.a)
			return 2
		},

		0x50: func(sys *cpu) int {
			sys.bit(2, sys.b)
			return 2
		},
		0x51: func(sys *cpu) int {
			sys.bit(2, sys.c)
			return 2
		},
		0x52: func(sys *cpu) int {
			sys.bit(2, sys.d)
			return 2
		},
		0x53: func(sys *cpu) int {
			sys.bit(2, sys.e)
			return 2
		},
		0x54: func(sys *cpu) int {
			sys.bit(2, sys.h())
			return 2
		},
		0x55: func(sys *cpu) int {
			sys.bit(2, sys.l())
			return 2
		},
		0x56: func(sys *cpu) int {
			sys.bit(2, sys.readByte(sys.hl))
			return 4
		},
		0x57: func(sys *cpu) int {
			sys.bit(2, sys.a)
			return 2
		},

		0x58: func(sys *cpu) int {
			sys.bit(3, sys.b)
			return 2
		},
		0x59: func(sys *cpu) int {
			sys.bit(3, sys.c)
			return 2
		},
		0x5A: func(sys *cpu) int {
			sys.bit(3, sys.d)
			return 2
		},
		0x5B: func(sys *cpu) int {
			sys.bit(3, sys.e)
			return 2
		},
		0x5C: func(sys *cpu) int {
			sys.bit(3, sys.h())
			return 2
		},
		0x5D: func(sys *cpu) int {
			sys.bit(3, sys.l())
			return 2
		},
		0x5E: func(sys *cpu) int {
			sys.bit(3, sys.readByte(sys.hl))
			return 4
		},
		0x5F: func(sys *cpu) int {
			sys.bit(3, sys.a)
			return 2
		},

		0x60: func(sys *cpu) int {
			sys.bit(4, sys.b)
			return 2
		},
		0x61: func(sys *cpu) int {
			sys.bit(4, sys.c)
			return 2
		},
		0x62: func(sys *cpu) int {
			sys.bit(4, sys.d)
			return 2
		},
		0x63: func(sys *cpu) int {
			sys.bit(4, sys.e)
			return 2
		},
		0x64: func(sys *cpu) int {
			sys.bit(4, sys.h())
			return 2
		},
		0x65: func(sys *cpu) int {
			sys.bit(4, sys.l())
			return 2
		},
		0x66: func(sys *cpu) int {
			sys.bit(4, sys.readByte(sys.hl))
			return 4
		},
		0x67: func(sys *cpu) int {
			sys.bit(4, sys.a)
			return 2
		},

		0x68: func(sys *cpu) int {
			sys.bit(5, sys.b)
			return 2
		},
		0x69: func(sys *cpu) int {
			sys.bit(5, sys.c)
			return 2
		},
		0x6A: func(sys *cpu) int {
			sys.bit(5, sys.d)
			return 2
		},
		0x6B: func(sys *cpu) int {
			sys.bit(5, sys.e)
			return 2
		},
		0x6C: func(sys *cpu) int {
			sys.bit(5, sys.h())
			return 2
		},
		0x6D: func(sys *cpu) int {
			sys.bit(5, sys.l())
			return 2
		},
		0x6E: func(sys *cpu) int {
			sys.bit(5, sys.readByte(sys.hl))
			return 4
		},
		0x6F: func(sys *cpu) int {
			sys.bit(5, sys.a)
			return 2
		},

		0x70: func(sys *cpu) int {
			sys.bit(6, sys.b)
			return 2
		},
		0x71: func(sys *cpu) int {
			sys.bit(6, sys.c)
			return 2
		},
		0x72: func(sys *cpu) int {
			sys.bit(6, sys.d)
			return 2
		},
		0x73: func(sys *cpu) int {
			sys.bit(6, sys.e)
			return 2
		},
		0x74: func(sys *cpu) int {
			sys.bit(6, sys.h())
			return 2
		},
		0x75: func(sys *cpu) int {
			sys.bit(6, sys.l())
			return 2
		},
		0x76: func(sys *cpu) int {
			sys.bit(6, sys.readByte(sys.hl))
			return 4
		},
		0x77: func(sys *cpu) int {
			sys.bit(6, sys.a)
			return 2
		},

		0x78: func(sys *cpu) int {
			sys.bit(7, sys.b)
			return 2
		},
		0x79: func(sys *cpu) int {
			sys.bit(7, sys.c)
			return 2
		},
		0x7A: func(sys *cpu) int {
			sys.bit(7, sys.d)
			return 2
		},
		0x7B: func(sys *cpu) int {
			sys.bit(7, sys.e)
			return 2
		},
		0x7C: func(sys *cpu) int {
			sys.bit(7, sys.h())
			return 2
		},
		0x7D: func(sys *cpu) int {
			sys.bit(7, sys.l())
			return 2
		},
		0x7E: func(sys *cpu) int {
			sys.bit(7, sys.readByte(sys.hl))
			return 4
		},
		0x7F: func(sys *cpu) int {
			sys.bit(7, sys.a)
			return 2
		},

		0x80: func(sys *cpu) int {
			sys.b = res(0, sys.b)
			return 2
		},
		0x81: func(sys *cpu) int {
			sys.c = res(0, sys.c)
			return 2
		},
		0x82: func(sys *cpu) int {
			sys.d = res(0, sys.d)
			return 2
		},
		0x83: func(sys *cpu) int {
			sys.e = res(0, sys.e)
			return 2
		},
		0x84: func(sys *cpu) int {
			sys.wh(res(0, sys.h()))
			return 2
		},
		0x85: func(sys *cpu) int {
			sys.wl(res(0, sys.l()))
			return 2
		},
		0x86: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, res(0, x))
			return 4
		},
		0x87: func(sys *cpu) int {
			sys.a = res(0, sys.a)
			return 2
		},

		0x88: func(sys *cpu) int {
			sys.b = res(1, sys.b)
			return 2
		},
		0x89: func(sys *cpu) int {
			sys.c = res(1, sys.c)
			return 2
		},
		0x8A: func(sys *cpu) int {
			sys.d = res(1, sys.d)
			return 2
		},
		0x8B: func(sys *cpu) int {
			sys.e = res(1, sys.e)
			return 2
		},
		0x8C: func(sys *cpu) int {
			sys.wh(res(1, sys.h()))
			return 2
		},
		0x8D: func(sys *cpu) int {
			sys.wl(res(1, sys.l()))
			return 2
		},
		0x8E: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, res(1, x))
			return 4
		},
		0x8F: func(sys *cpu) int {
			sys.a = res(1, sys.a)
			return 2
		},

		0x90: func(sys *cpu) int {
			sys.b = res(2, sys.b)
			return 2
		},
		0x91: func(sys *cpu) int {
			sys.c = res(2, sys.c)
			return 2
		},
		0x92: func(sys *cpu) int {
			sys.d = res(2, sys.d)
			return 2
		},
		0x93: func(sys *cpu) int {
			sys.e = res(2, sys.e)
			return 2
		},
		0x94: func(sys *cpu) int {
			sys.wh(res(2, sys.h()))
			return 2
		},
		0x95: func(sys *cpu) int {
			sys.wl(res(2, sys.l()))
			return 2
		},
		0x96: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, res(2, x))
			return 4
		},
		0x97: func(sys *cpu) int {
			sys.a = res(2, sys.a)
			return 2
		},

		0x98: func(sys *cpu) int {
			sys.b = res(3, sys.b)
			return 2
		},
		0x99: func(sys *cpu) int {
			sys.c = res(3, sys.c)
			return 2
		},
		0x9A: func(sys *cpu) int {
			sys.d = res(3, sys.d)
			return 2
		},
		0x9B: func(sys *cpu) int {
			sys.e = res(3, sys.e)
			return 2
		},
		0x9C: func(sys *cpu) int {
			sys.wh(res(3, sys.h()))
			return 2
		},
		0x9D: func(sys *cpu) int {
			sys.wl(res(3, sys.l()))
			return 2
		},
		0x9E: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, res(3, x))
			return 4
		},
		0x9F: func(sys *cpu) int {
			sys.a = res(3, sys.a)
			return 2
		},

		0xA0: func(sys *cpu) int {
			sys.b = res(4, sys.b)
			return 2
		},
		0xA1: func(sys *cpu) int {
			sys.c = res(4, sys.c)
			return 2
		},
		0xA2: func(sys *cpu) int {
			sys.d = res(4, sys.d)
			return 2
		},
		0xA3: func(sys *cpu) int {
			sys.e = res(4, sys.e)
			return 2
		},
		0xA4: func(sys *cpu) int {
			sys.wh(res(4, sys.h()))
			return 2
		},
		0xA5: func(sys *cpu) int {
			sys.wl(res(4, sys.l()))
			return 2
		},
		0xA6: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, res(4, x))
			return 4
		},
		0xA7: func(sys *cpu) int {
			sys.a = res(4, sys.a)
			return 2
		},

		0xA8: func(sys *cpu) int {
			sys.b = res(5, sys.b)
			return 2
		},
		0xA9: func(sys *cpu) int {
			sys.c = res(5, sys.c)
			return 2
		},
		0xAA: func(sys *cpu) int {
			sys.d = res(5, sys.d)
			return 2
		},
		0xAB: func(sys *cpu) int {
			sys.e = res(5, sys.e)
			return 2
		},
		0xAC: func(sys *cpu) int {
			sys.wh(res(5, sys.h()))
			return 2
		},
		0xAD: func(sys *cpu) int {
			sys.wl(res(5, sys.l()))
			return 2
		},
		0xAE: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, res(5, x))
			return 4
		},
		0xAF: func(sys *cpu) int {
			sys.a = res(5, sys.a)
			return 2
		},

		0xB0: func(sys *cpu) int {
			sys.b = res(6, sys.b)
			return 2
		},
		0xB1: func(sys *cpu) int {
			sys.c = res(6, sys.c)
			return 2
		},
		0xB2: func(sys *cpu) int {
			sys.d = res(6, sys.d)
			return 2
		},
		0xB3: func(sys *cpu) int {
			sys.e = res(6, sys.e)
			return 2
		},
		0xB4: func(sys *cpu) int {
			sys.wh(res(6, sys.h()))
			return 2
		},
		0xB5: func(sys *cpu) int {
			sys.wl(res(6, sys.l()))
			return 2
		},
		0xB6: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, res(6, x))
			return 4
		},
		0xB7: func(sys *cpu) int {
			sys.a = res(6, sys.a)
			return 2
		},

		0xB8: func(sys *cpu) int {
			sys.b = res(7, sys.b)
			return 2
		},
		0xB9: func(sys *cpu) int {
			sys.c = res(7, sys.c)
			return 2
		},
		0xBA: func(sys *cpu) int {
			sys.d = res(7, sys.d)
			return 2
		},
		0xBB: func(sys *cpu) int {
			sys.e = res(7, sys.e)
			return 2
		},
		0xBC: func(sys *cpu) int {
			sys.wh(res(7, sys.h()))
			return 2
		},
		0xBD: func(sys *cpu) int {
			sys.wl(res(7, sys.l()))
			return 2
		},
		0xBE: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, res(7, x))
			return 4
		},
		0xBF: func(sys *cpu) int {
			sys.a = res(7, sys.a)
			return 2
		},

		0xC0: func(sys *cpu) int {
			sys.b = set(0, sys.b)
			return 2
		},
		0xC1: func(sys *cpu) int {
			sys.c = set(0, sys.c)
			return 2
		},
		0xC2: func(sys *cpu) int {
			sys.d = set(0, sys.d)
			return 2
		},
		0xC3: func(sys *cpu) int {
			sys.e = set(0, sys.e)
			return 2
		},
		0xC4: func(sys *cpu) int {
			sys.wh(set(0, sys.h()))
			return 2
		},
		0xC5: func(sys *cpu) int {
			sys.wl(set(0, sys.l()))
			return 2
		},
		0xC6: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, set(0, x))
			return 4
		},
		0xC7: func(sys *cpu) int {
			sys.a = set(0, sys.a)
			return 2
		},

		0xC8: func(sys *cpu) int {
			sys.b = set(1, sys.b)
			return 2
		},
		0xC9: func(sys *cpu) int {
			sys.c = set(1, sys.c)
			return 2
		},
		0xCA: func(sys *cpu) int {
			sys.d = set(1, sys.d)
			return 2
		},
		0xCB: func(sys *cpu) int {
			sys.e = set(1, sys.e)
			return 2
		},
		0xCC: func(sys *cpu) int {
			sys.wh(set(1, sys.h()))
			return 2
		},
		0xCD: func(sys *cpu) int {
			sys.wl(set(1, sys.l()))
			return 2
		},
		0xCE: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, set(1, x))
			return 4
		},
		0xCF: func(sys *cpu) int {
			sys.a = set(1, sys.a)
			return 2
		},

		0xD0: func(sys *cpu) int {
			sys.b = set(2, sys.b)
			return 2
		},
		0xD1: func(sys *cpu) int {
			sys.c = set(2, sys.c)
			return 2
		},
		0xD2: func(sys *cpu) int {
			sys.d = set(2, sys.d)
			return 2
		},
		0xD3: func(sys *cpu) int {
			sys.e = set(2, sys.e)
			return 2
		},
		0xD4: func(sys *cpu) int {
			sys.wh(set(2, sys.h()))
			return 2
		},
		0xD5: func(sys *cpu) int {
			sys.wl(set(2, sys.l()))
			return 2
		},
		0xD6: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, set(2, x))
			return 4
		},
		0xD7: func(sys *cpu) int {
			sys.a = set(2, sys.a)
			return 2
		},

		0xD8: func(sys *cpu) int {
			sys.b = set(3, sys.b)
			return 2
		},
		0xD9: func(sys *cpu) int {
			sys.c = set(3, sys.c)
			return 2
		},
		0xDA: func(sys *cpu) int {
			sys.d = set(3, sys.d)
			return 2
		},
		0xDB: func(sys *cpu) int {
			sys.e = set(3, sys.e)
			return 2
		},
		0xDC: func(sys *cpu) int {
			sys.wh(set(3, sys.h()))
			return 2
		},
		0xDD: func(sys *cpu) int {
			sys.wl(set(3, sys.l()))
			return 2
		},
		0xDE: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, set(3, x))
			return 4
		},
		0xDF: func(sys *cpu) int {
			sys.a = set(3, sys.a)
			return 2
		},

		0xE0: func(sys *cpu) int {
			sys.b = set(4, sys.b)
			return 2
		},
		0xE1: func(sys *cpu) int {
			sys.c = set(4, sys.c)
			return 2
		},
		0xE2: func(sys *cpu) int {
			sys.d = set(4, sys.d)
			return 2
		},
		0xE3: func(sys *cpu) int {
			sys.e = set(4, sys.e)
			return 2
		},
		0xE4: func(sys *cpu) int {
			sys.wh(set(4, sys.h()))
			return 2
		},
		0xE5: func(sys *cpu) int {
			sys.wl(set(4, sys.l()))
			return 2
		},
		0xE6: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, set(4, x))
			return 4
		},
		0xE7: func(sys *cpu) int {
			sys.a = set(4, sys.a)
			return 2
		},

		0xE8: func(sys *cpu) int {
			sys.b = set(5, sys.b)
			return 2
		},
		0xE9: func(sys *cpu) int {
			sys.c = set(5, sys.c)
			return 2
		},
		0xEA: func(sys *cpu) int {
			sys.d = set(5, sys.d)
			return 2
		},
		0xEB: func(sys *cpu) int {
			sys.e = set(5, sys.e)
			return 2
		},
		0xEC: func(sys *cpu) int {
			sys.wh(set(5, sys.h()))
			return 2
		},
		0xED: func(sys *cpu) int {
			sys.wl(set(5, sys.l()))
			return 2
		},
		0xEE: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, set(5, x))
			return 4
		},
		0xEF: func(sys *cpu) int {
			sys.a = set(5, sys.a)
			return 2
		},

		0xF0: func(sys *cpu) int {
			sys.b = set(6, sys.b)
			return 2
		},
		0xF1: func(sys *cpu) int {
			sys.c = set(6, sys.c)
			return 2
		},
		0xF2: func(sys *cpu) int {
			sys.d = set(6, sys.d)
			return 2
		},
		0xF3: func(sys *cpu) int {
			sys.e = set(6, sys.e)
			return 2
		},
		0xF4: func(sys *cpu) int {
			sys.wh(set(6, sys.h()))
			return 2
		},
		0xF5: func(sys *cpu) int {
			sys.wl(set(6, sys.l()))
			return 2
		},
		0xF6: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, set(6, x))
			return 4
		},
		0xF7: func(sys *cpu) int {
			sys.a = set(6, sys.a)
			return 2
		},

		0xF8: func(sys *cpu) int {
			sys.b = set(7, sys.b)
			return 2
		},
		0xF9: func(sys *cpu) int {
			sys.c = set(7, sys.c)
			return 2
		},
		0xFA: func(sys *cpu) int {
			sys.d = set(7, sys.d)
			return 2
		},
		0xFB: func(sys *cpu) int {
			sys.e = set(7, sys.e)
			return 2
		},
		0xFC: func(sys *cpu) int {
			sys.wh(set(7, sys.h()))
			return 2
		},
		0xFD: func(sys *cpu) int {
			sys.wl(set(7, sys.l()))
			return 2
		},
		0xFE: func(sys *cpu) int {
			x := sys.readByte(sys.hl)
			sys.writeByte(sys.hl, set(7, x))
			return 4
		},
		0xFF: func(sys *cpu) int {
			sys.a = set(7, sys.a)
			return 2
		},
	}
}

func (m *memory) disasm(addr uint16) (result string) {
	defer func() {
		if e := recover(); e != nil {
			result = "(read failed!)"
		}
	}()
	code := m.readByte(addr)
	imm8 := m.readByte(addr + 1)
	imm16 := m.readWord(addr + 1)
	rel8 := int8(imm8)
	jra := addr + 2 + uint16(rel8)
	switch code {
	case 0x00:
		return "NOP"
	case 0x01:
		return fmt.Sprintf("LD BC,%04Xh", imm16)
	case 0x02:
		return "LD (BC),A"
	case 0x03:
		return "INC BC"
	case 0x04:
		return "INC B"
	case 0x05:
		return "DEC B"
	case 0x06:
		return fmt.Sprintf("LD B,%02Xh", imm8)
	case 0x07:
		return "RLCA"
	case 0x08:
		return fmt.Sprintf("LD (%04Xh),SP", imm16)
	case 0x09:
		return "ADD HL,BC"
	case 0x0A:
		return "LD A,(BC)"
	case 0x0B:
		return "DEC BC"
	case 0x0C:
		return "INC C"
	case 0x0D:
		return "DEC C"
	case 0x0E:
		return fmt.Sprintf("LD C,%02Xh", imm8)
	case 0x0F:
		return "RRCA"
	case 0x10:
		return "STOP"
	case 0x11:
		return fmt.Sprintf("LD DE,%04Xh", imm16)
	case 0x12:
		return "LD (DE),A"
	case 0x13:
		return "INC DE"
	case 0x14:
		return "INC D"
	case 0x15:
		return "DEC D"
	case 0x16:
		return fmt.Sprintf("LD D,%02Xh", imm8)
	case 0x17:
		return "RLA"
	case 0x18:
		return fmt.Sprintf("JR %04Xh", jra)
	case 0x19:
		return "ADD HL,DE"
	case 0x1A:
		return "LD A,(DE)"
	case 0x1B:
		return "DEC DE"
	case 0x1C:
		return "INC E"
	case 0x1D:
		return "DEC E"
	case 0x1E:
		return fmt.Sprintf("LD E,%02Xh", imm8)
	case 0x1F:
		return "RRA"
	case 0x20:
		return fmt.Sprintf("JR NZ,%04Xh", jra)
	case 0x21:
		return fmt.Sprintf("LD HL,%04Xh", imm16)
	case 0x22:
		return "LD (HL+),A"
	case 0x23:
		return "INC HL"
	case 0x24:
		return "INC H"
	case 0x25:
		return "DEC H"
	case 0x26:
		return fmt.Sprintf("LD H,%02Xh", imm8)
	case 0x27:
		return "DAA"
	case 0x28:
		return fmt.Sprintf("JR Z,%04Xh", jra)
	case 0x29:
		return "ADD HL,HL"
	case 0x2A:
		return "LD A,(HL+)"
	case 0x2B:
		return "DEC HL"
	case 0x2C:
		return "INC L"
	case 0x2D:
		return "DEC L"
	case 0x2E:
		return fmt.Sprintf("LD L,%02Xh", imm8)
	case 0x2F:
		return "CPL"
	case 0x30:
		return fmt.Sprintf("JR NC,%04Xh", jra)
	case 0x31:
		return fmt.Sprintf("LD SP,%04Xh", imm16)
	case 0x32:
		return "LD (HL-),A"
	case 0x33:
		return "INC SP"
	case 0x34:
		return "INC (HL)"
	case 0x35:
		return "DEC (HL)"
	case 0x36:
		return fmt.Sprintf("LD (HL),%02Xh", imm8)
	case 0x37:
		return "SCF"
	case 0x38:
		return fmt.Sprintf("JR C,%04Xh", jra)
	case 0x39:
		return "ADD HL,SP"
	case 0x3A:
		return "LD A,(HL-)"
	case 0x3B:
		return "DEC SP"
	case 0x3C:
		return "INC A"
	case 0x3D:
		return "DEC A"
	case 0x3E:
		return fmt.Sprintf("LD A,%02Xh", imm8)
	case 0x3F:
		return "CCF"
	case 0x40:
		return "LD B,B"
	case 0x41:
		return "LD B,C"
	case 0x42:
		return "LD B,D"
	case 0x43:
		return "LD B,E"
	case 0x44:
		return "LD B,H"
	case 0x45:
		return "LD B.L"
	case 0x46:
		return "LD B,(HL)"
	case 0x47:
		return "LD B,A"
	case 0x48:
		return "LD C,B"
	case 0x49:
		return "LD C,C"
	case 0x4A:
		return "LD C,D"
	case 0x4B:
		return "LD C,E"
	case 0x4C:
		return "LD C,H"
	case 0x4D:
		return "LD C,L"
	case 0x4E:
		return "LD C,(HL)"
	case 0x4F:
		return "LD C,A"
	case 0x50:
		return "LD D,B"
	case 0x51:
		return "LD D,C"
	case 0x52:
		return "LD D,D"
	case 0x53:
		return "LD D,E"
	case 0x54:
		return "LD D,H"
	case 0x55:
		return "LD D,L"
	case 0x56:
		return "LD D,(HL)"
	case 0x57:
		return "LD D,A"
	case 0x58:
		return "LD E,B"
	case 0x59:
		return "LD E,C"
	case 0x5A:
		return "LD E,D"
	case 0x5B:
		return "LD E,E"
	case 0x5C:
		return "LD E,H"
	case 0x5D:
		return "LD E,L"
	case 0x5E:
		return "LD E,(HL)"
	case 0x5F:
		return "LD E,A"
	case 0x60:
		return "LD H,B"
	case 0x61:
		return "LD H,C"
	case 0x62:
		return "LD H,D"
	case 0x63:
		return "LD H,E"
	case 0x64:
		return "LD H,H"
	case 0x65:
		return "LD H,L"
	case 0x66:
		return "LD H,(HL)"
	case 0x67:
		return "LD H,A"
	case 0x68:
		return "LD L,B"
	case 0x69:
		return "LD L,C"
	case 0x6A:
		return "LD L,D"
	case 0x6B:
		return "LD L,E"
	case 0x6C:
		return "LD L,H"
	case 0x6D:
		return "LD L,L"
	case 0x6E:
		return "LD L,(HL)"
	case 0x6F:
		return "LD L,A"
	case 0x70:
		return "LD (HL),B"
	case 0x71:
		return "LD (HL),c"
	case 0x72:
		return "LD (HL),D"
	case 0x73:
		return "LD (HL),E"
	case 0x74:
		return "LD (HL),H"
	case 0x75:
		return "LD (HL),L"
	case 0x76:
		return "HALT"
	case 0x77:
		return "LD (HL),A"
	case 0x78:
		return "LD A,B"
	case 0x79:
		return "LD A,C"
	case 0x7A:
		return "LD A,D"
	case 0x7B:
		return "LD A,E"
	case 0x7C:
		return "LD A,H"
	case 0x7D:
		return "LD A,L"
	case 0x7E:
		return "LD A,(HL)"
	case 0x7F:
		return "LD A,A"
	case 0x80:
		return "ADD A,B"
	case 0x81:
		return "ADD A,C"
	case 0x82:
		return "ADD A,D"
	case 0x83:
		return "ADD A,E"
	case 0x84:
		return "ADD A,H"
	case 0x85:
		return "ADD A,L"
	case 0x86:
		return "ADD A,(HL)"
	case 0x87:
		return "ADD A,A"
	case 0x88:
		return "ADC A,B"
	case 0x89:
		return "ADC A,C"
	case 0x8A:
		return "ADC A,D"
	case 0x8B:
		return "ADC A,E"
	case 0x8C:
		return "ADC A,H"
	case 0x8D:
		return "ADC A,L"
	case 0x8E:
		return "ADC A,(HL)"
	case 0x8F:
		return "ADC A,A"
	case 0x90:
		return "SUB B"
	case 0x91:
		return "SUB C"
	case 0x92:
		return "SUB D"
	case 0x93:
		return "SUB E"
	case 0x94:
		return "SUB H"
	case 0x95:
		return "SUB L"
	case 0x96:
		return "SUB (HL)"
	case 0x97:
		return "SUB A"
	case 0x98:
		return "SBC B"
	case 0x99:
		return "SBC C"
	case 0x9A:
		return "SBC D"
	case 0x9B:
		return "SBC E"
	case 0x9C:
		return "SBC H"
	case 0x9D:
		return "SBC L"
	case 0x9E:
		return "SBC (HL)"
	case 0x9F:
		return "SBC A"
	case 0xA0:
		return "AND B"
	case 0xA1:
		return "AND C"
	case 0xA2:
		return "AND D"
	case 0xA3:
		return "AND E"
	case 0xA4:
		return "AND H"
	case 0xA5:
		return "AND L"
	case 0xA6:
		return "AND (HL)"
	case 0xA7:
		return "AND A"
	case 0xA8:
		return "XOR B"
	case 0xA9:
		return "XOR C"
	case 0xAA:
		return "XOR D"
	case 0xAB:
		return "XOR E"
	case 0xAC:
		return "XOR H"
	case 0xAD:
		return "XOR L"
	case 0xAE:
		return "XOR (HL)"
	case 0xAF:
		return "XOR A"
	case 0xB0:
		return "OR B"
	case 0xB1:
		return "OR C"
	case 0xB2:
		return "OR D"
	case 0xB3:
		return "OR E"
	case 0xB4:
		return "OR H"
	case 0xB5:
		return "OR L"
	case 0xB6:
		return "OR (HL)"
	case 0xB7:
		return "OR A"
	case 0xB8:
		return "CP B"
	case 0xB9:
		return "CP C"
	case 0xBA:
		return "CP D"
	case 0xBB:
		return "CP E"
	case 0xBC:
		return "CP H"
	case 0xBD:
		return "CP L"
	case 0xBE:
		return "CP (HL)"
	case 0xBF:
		return "CP A"
	case 0xC0:
		return "RET NZ"
	case 0xC1:
		return "POP BC"
	case 0xC2:
		return fmt.Sprintf("JP NZ,%04Xh", imm16)
	case 0xC3:
		return fmt.Sprintf("JP %04Xh", imm16)
	case 0xC4:
		return fmt.Sprintf("CALL NZ,%04Xh", imm16)
	case 0xC5:
		return "PUSH BC"
	case 0xC6:
		return fmt.Sprintf("ADD A,%02Xh", imm8)
	case 0xC7:
		return "RST 00h"
	case 0xC8:
		return "RET Z"
	case 0xC9:
		return "RET"
	case 0xCA:
		return fmt.Sprintf("JP Z,%04Xh", imm16)
	case 0xCB:
		regs := [...]string{"B", "C", "D", "E", "H", "L", "(HL)", "A"}
		reg := regs[imm8&7]
		bit := (imm8 >> 3) & 7
		switch {
		case imm8 < 0x08:
			return fmt.Sprintf("RLC %s", reg)
		case imm8 < 0x10:
			return fmt.Sprintf("RRC %s", reg)
		case imm8 < 0x18:
			return fmt.Sprintf("RL %s", reg)
		case imm8 < 0x20:
			return fmt.Sprintf("RR %s", reg)
		case imm8 < 0x28:
			return fmt.Sprintf("SLA %s", reg)
		case imm8 < 0x30:
			return fmt.Sprintf("SRA %s", reg)
		case imm8 < 0x38:
			return fmt.Sprintf("SWAP %s", reg)
		case imm8 < 0x40:
			return fmt.Sprintf("SRL %s", reg)
		case imm8 < 0x80:
			return fmt.Sprintf("BIT %d,%s", bit, reg)
		case imm8 < 0xC0:
			return fmt.Sprintf("RES %d,%s", bit, reg)
		default:
			return fmt.Sprintf("SET %d,%s", bit, reg)
		}
	case 0xCC:
		return fmt.Sprintf("CALL Z,%04Xh", imm16)
	case 0xCD:
		return fmt.Sprintf("CALL %04Xh", imm16)
	case 0xCE:
		return fmt.Sprintf("ADC A,%02Xh", imm8)
	case 0xCF:
		return "RST 08h"
	case 0xD0:
		return "RET NC"
	case 0xD1:
		return "POP DE"
	case 0xD2:
		return fmt.Sprintf("JP NC,%04Xh", imm16)
	case 0xD4:
		return fmt.Sprintf("CALL NC,%04Xh", imm16)
	case 0xD5:
		return "PUSH DE"
	case 0xD6:
		return fmt.Sprintf("SUB %02Xh", imm8)
	case 0xD7:
		return "RST 10h"
	case 0xD8:
		return "RET C"
	case 0xD9:
		return "RETI"
	case 0xDA:
		return fmt.Sprintf("JP C,%04Xh", imm16)
	case 0xDC:
		return fmt.Sprintf("CALL C,%04Xh", imm16)
	case 0xDE:
		return fmt.Sprintf("SBC %02Xh", imm8)
	case 0xDF:
		return "RST 18h"
	case 0xE0:
		return fmt.Sprintf("LDH (%02Xh),A", imm8)
	case 0xE1:
		return "POP HL"
	case 0xE2:
		return "LD (C),A"
	case 0xE5:
		return "PUSH HL"
	case 0xE6:
		return fmt.Sprintf("AND %02Xh", imm8)
	case 0xE7:
		return "RST 20h"
	case 0xE8:
		return fmt.Sprintf("ADD SP,%d", rel8)
	case 0xE9:
		return "JP (HL)"
	case 0xEA:
		return fmt.Sprintf("LD (%04Xh),A", imm16)
	case 0xEE:
		return fmt.Sprintf("XOR %02Xh", imm8)
	case 0xEF:
		return "RST 28h"
	case 0xF0:
		return fmt.Sprintf("LDH A,(%02Xh)", imm8)
	case 0xF1:
		return "POP AF"
	case 0xF2:
		return "LD A,(C)"
	case 0xF3:
		return "DI"
	case 0xF5:
		return "PUSH AF"
	case 0xF6:
		return fmt.Sprintf("OR %02Xh", imm8)
	case 0xF7:
		return "RST 30h"
	case 0xF8:
		return fmt.Sprintf("LD HL,SP%+d", rel8)
	case 0xF9:
		return "LD SP,HL"
	case 0xFA:
		return fmt.Sprintf("LD A,(%04Xh)", imm16)
	case 0xFB:
		return "EI"
	case 0xFE:
		return fmt.Sprintf("CP %02Xh", imm8)
	case 0xFF:
		return "RST 38h"
	}
	return fmt.Sprintf("%02Xh", code)
}
