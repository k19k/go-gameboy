package gameboy

import (
	"fmt"
	"io"
)

type cpu struct {
	*memory
	a, b, c, d, e byte
	hl, pc, sp uint16
	fz, fn, fh, fc bool
	ime, halt, pause bool
	mar uint16
	stack uint16
	trace [256]uint16
	traceptr byte
}

func newCPU(m *memory) *cpu {
	return &cpu { memory: m,
	        a: 0x01, b: 0x00, c: 0x13, d: 0x00, e: 0xD8,
		hl: 0x014D, pc: 0x0100, sp: 0xFFFE,
		fz: true, fn: false, fh: true, fc: true,
		ime: true, halt: false, pause: true,
		mar: 0x0100, stack: 0xFFFE }
}

func (sys *cpu) String() string {
	return fmt.Sprintf(
		"<cpu AF=%04X BC=%04X DE=%04X HL=%04X\n" +
		"     PC=%04X SP=%04X\n" +
		"     IME=%t Halt=%t Pause=%t>",
		sys.af(), sys.bc(), sys.de(), sys.hl,
		sys.pc, sys.sp, sys.ime, sys.halt, sys.pause)
}

func (sys *cpu) step() int {
	t := 4
	if !sys.halt {
		if sys.pc >= 0x8000 && sys.pc < 0xFF80 &&
			sys.pc < 0xC000 && sys.pc >= 0xFE00 {
			panic("executing data")
		}
		if sys.stack - sys.sp >= 0x7E {
			panic("stack overflow")
		}
		sys.mar = sys.pc
		sys.trace[sys.traceptr] = sys.pc
		sys.traceptr++
		//fmt.Printf("%04X %s\n", sys.pc, sys.disasm(sys.pc))
		t = sys.fdx()
	}
	if sys.ime {
		f := sys.readPort(portIF)
		e := sys.readPort(portIE)
		mask := f & e & 0x1F
		if mask != 0 {
			t += sys.irq(mask, f)
		}
	}
	sys.updateTimers(t)
	return t
}

func (sys *cpu) irq(mask, f byte) int {
	sys.ime = false
	sys.halt = false
	sys.push(sys.pc)
	if mask & 0x01 != 0 {
		sys.pc = vblankAddr
		f &^= 0x01
	} else if mask & 0x02 != 0 {
		sys.pc = lcdStatusAddr
		f &^= 0x02
	} else if mask & 0x04 != 0 {
		sys.pc = timerAddr
		f &^= 0x04
	} else if mask & 0x08 != 0 {
		sys.pc = serialAddr
		f &^= 0x08
	} else if mask & 0x10 != 0 {
		sys.pc = joypadAddr
		f &^= 0x10
	}
	sys.writePort(portIF, f)
	return 32/4
}

func (sys *cpu) dumpStack(w io.Writer) {
	addr := sys.stack-2
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

func (sys *cpu) traceback(w io.Writer) {
	fmt.Fprintln(w, "EXECUTION TRACE")
	for i := 255; i >= 0; i-- {
		addr := sys.trace[sys.traceptr]
		fmt.Fprintf(w, "%3d %04X %s\n", i, addr, sys.disasm(addr))
		sys.traceptr++
	}
	fmt.Fprintln(w)
}

func (m *memory) disasm(addr uint16) (result string) {
	defer func() {
		if e := recover(); e != nil {
			result = "(read failed!)"
		}
	}()
	code := m.readByte(addr)
	imm8 := m.readByte(addr+1)
	imm16 := m.readWord(addr+1)
	rel8 := int8(imm8)
	jra := addr + 2 + uint16(rel8)
	switch code {
	case 0x00: return "NOP"
	case 0x01: return fmt.Sprintf("LD BC,%04Xh", imm16)
	case 0x02: return "LD (BC),A"
	case 0x03: return "INC BC"
	case 0x04: return "INC B"
	case 0x05: return "DEC B"
	case 0x06: return fmt.Sprintf("LD B,%02Xh", imm8)
	case 0x07: return "RLCA"
	case 0x08: return fmt.Sprintf("LD (%04Xh),SP", imm16)
	case 0x09: return "ADD HL,BC"
	case 0x0A: return "LD A,(BC)"
	case 0x0B: return "DEC BC"
	case 0x0C: return "INC C"
	case 0x0D: return "DEC C"
	case 0x0E: return fmt.Sprintf("LD C,%02Xh", imm8)
	case 0x0F: return "RRCA"
	case 0x10: return "STOP"
	case 0x11: return fmt.Sprintf("LD DE,%04Xh", imm16)
	case 0x12: return "LD (DE),A"
	case 0x13: return "INC DE"
	case 0x14: return "INC D"
	case 0x15: return "DEC D"
	case 0x16: return fmt.Sprintf("LD D,%02Xh", imm8)
	case 0x17: return "RLA"
	case 0x18: return fmt.Sprintf("JR %04Xh", jra)
	case 0x19: return "ADD HL,DE"
	case 0x1A: return "LD A,(DE)"
	case 0x1B: return "DEC DE"
	case 0x1C: return "INC E"
	case 0x1D: return "DEC E"
	case 0x1E: return fmt.Sprintf("LD E,%02Xh", imm8)
	case 0x1F: return "RRA"
	case 0x20: return fmt.Sprintf("JR NZ,%04Xh", jra)
	case 0x21: return fmt.Sprintf("LD HL,%04Xh", imm16)
	case 0x22: return "LD (HL+),A"
	case 0x23: return "INC HL"
	case 0x24: return "INC H"
	case 0x25: return "DEC H"
	case 0x26: return fmt.Sprintf("LD H,%02Xh", imm8)
	case 0x27: return "DAA"
	case 0x28: return fmt.Sprintf("JR Z,%04Xh", jra)
	case 0x29: return "ADD HL,HL"
	case 0x2A: return "LD A,(HL+)"
	case 0x2B: return "DEC HL"
	case 0x2C: return "INC L"
	case 0x2D: return "DEC L"
	case 0x2E: return fmt.Sprintf("LD L,%02Xh", imm8)
	case 0x2F: return "CPL"
	case 0x30: return fmt.Sprintf("JR NC,%04Xh", jra)
	case 0x31: return fmt.Sprintf("LD SP,%04Xh", imm16)
	case 0x32: return "LD (HL-),A"
	case 0x33: return "INC SP"
	case 0x34: return "INC (HL)"
	case 0x35: return "DEC (HL)"
	case 0x36: return fmt.Sprintf("LD (HL),%02Xh", imm8)
	case 0x37: return "SCF"
	case 0x38: return fmt.Sprintf("JR C,%04Xh", jra)
	case 0x39: return "ADD HL,SP"
	case 0x3A: return "LD A,(HL-)"
	case 0x3B: return "DEC SP"
	case 0x3C: return "INC A"
	case 0x3D: return "DEC A"
	case 0x3E: return fmt.Sprintf("LD A,%02Xh", imm8)
	case 0x3F: return "CCF"
	case 0x40: return "LD B,B"
	case 0x41: return "LD B,C"
	case 0x42: return "LD B,D"
	case 0x43: return "LD B,E"
	case 0x44: return "LD B,H"
	case 0x45: return "LD B.L"
	case 0x46: return "LD B,(HL)"
	case 0x47: return "LD B,A"
	case 0x48: return "LD C,B"
	case 0x49: return "LD C,C"
	case 0x4A: return "LD C,D"
	case 0x4B: return "LD C,E"
	case 0x4C: return "LD C,H"
	case 0x4D: return "LD C,L"
	case 0x4E: return "LD C,(HL)"
	case 0x4F: return "LD C,A"
	case 0x50: return "LD D,B"
	case 0x51: return "LD D,C"
	case 0x52: return "LD D,D"
	case 0x53: return "LD D,E"
	case 0x54: return "LD D,H"
	case 0x55: return "LD D,L"
	case 0x56: return "LD D,(HL)"
	case 0x57: return "LD D,A"
	case 0x58: return "LD E,B"
	case 0x59: return "LD E,C"
	case 0x5A: return "LD E,D"
	case 0x5B: return "LD E,E"
	case 0x5C: return "LD E,H"
	case 0x5D: return "LD E,L"
	case 0x5E: return "LD E,(HL)"
	case 0x5F: return "LD E,A"
	case 0x60: return "LD H,B"
	case 0x61: return "LD H,C"
	case 0x62: return "LD H,D"
	case 0x63: return "LD H,E"
	case 0x64: return "LD H,H"
	case 0x65: return "LD H,L"
	case 0x66: return "LD H,(HL)"
	case 0x67: return "LD H,A"
	case 0x68: return "LD L,B"
	case 0x69: return "LD L,C"
	case 0x6A: return "LD L,D"
	case 0x6B: return "LD L,E"
	case 0x6C: return "LD L,H"
	case 0x6D: return "LD L,L"
	case 0x6E: return "LD L,(HL)"
	case 0x6F: return "LD L,A"
	case 0x70: return "LD (HL),B"
	case 0x71: return "LD (HL),c"
	case 0x72: return "LD (HL),D"
	case 0x73: return "LD (HL),E"
	case 0x74: return "LD (HL),H"
	case 0x75: return "LD (HL),L"
	case 0x76: return "HALT"
	case 0x77: return "LD (HL),A"
	case 0x78: return "LD A,B"
	case 0x79: return "LD A,C"
	case 0x7A: return "LD A,D"
	case 0x7B: return "LD A,E"
	case 0x7C: return "LD A,H"
	case 0x7D: return "LD A,L"
	case 0x7E: return "LD A,(HL)"
	case 0x7F: return "LD A,A"
	case 0x80: return "ADD A,B"
	case 0x81: return "ADD A,C"
	case 0x82: return "ADD A,D"
	case 0x83: return "ADD A,E"
	case 0x84: return "ADD A,H"
	case 0x85: return "ADD A,L"
	case 0x86: return "ADD A,(HL)"
	case 0x87: return "ADD A,A"
	case 0x88: return "ADC A,B"
	case 0x89: return "ADC A,C"
	case 0x8A: return "ADC A,D"
	case 0x8B: return "ADC A,E"
	case 0x8C: return "ADC A,H"
	case 0x8D: return "ADC A,L"
	case 0x8E: return "ADC A,(HL)"
	case 0x8F: return "ADC A,A"
	case 0x90: return "SUB B"
	case 0x91: return "SUB C"
	case 0x92: return "SUB D"
	case 0x93: return "SUB E"
	case 0x94: return "SUB H"
	case 0x95: return "SUB L"
	case 0x96: return "SUB (HL)"
	case 0x97: return "SUB A"
	case 0x98: return "SBC B"
	case 0x99: return "SBC C"
	case 0x9A: return "SBC D"
	case 0x9B: return "SBC E"
	case 0x9C: return "SBC H"
	case 0x9D: return "SBC L"
	case 0x9E: return "SBC (HL)"
	case 0x9F: return "SBC A"
	case 0xA0: return "AND B"
	case 0xA1: return "AND C"
	case 0xA2: return "AND D"
	case 0xA3: return "AND E"
	case 0xA4: return "AND H"
	case 0xA5: return "AND L"
	case 0xA6: return "AND (HL)"
	case 0xA7: return "AND A"
	case 0xA8: return "XOR B"
	case 0xA9: return "XOR C"
	case 0xAA: return "XOR D"
	case 0xAB: return "XOR E"
	case 0xAC: return "XOR H"
	case 0xAD: return "XOR L"
	case 0xAE: return "XOR (HL)"
	case 0xAF: return "XOR A"
	case 0xB0: return "OR B"
	case 0xB1: return "OR C"
	case 0xB2: return "OR D"
	case 0xB3: return "OR E"
	case 0xB4: return "OR H"
	case 0xB5: return "OR L"
	case 0xB6: return "OR (HL)"
	case 0xB7: return "OR A"
	case 0xB8: return "CP B"
	case 0xB9: return "CP C"
	case 0xBA: return "CP D"
	case 0xBB: return "CP E"
	case 0xBC: return "CP H"
	case 0xBD: return "CP L"
	case 0xBE: return "CP (HL)"
	case 0xBF: return "CP A"
	case 0xC0: return "RET NZ"
	case 0xC1: return "POP BC"
	case 0xC2: return fmt.Sprintf("JP NZ,%04Xh", imm16)
	case 0xC3: return fmt.Sprintf("JP %04Xh", imm16)
	case 0xC4: return fmt.Sprintf("CALL NZ,%04Xh", imm16)
	case 0xC5: return "PUSH BC"
	case 0xC6: return fmt.Sprintf("ADD A,%02Xh", imm8)
	case 0xC7: return "RST 00h"
	case 0xC8: return "RET Z"
	case 0xC9: return "RET"
	case 0xCA: return fmt.Sprintf("JP Z,%04Xh", imm16)
	case 0xCB:
		regs := [...]string{"B","C","D","E","H","L","(HL)","A"}
		reg := regs[imm8 & 7]
		bit := (imm8 >> 3) & 7
		switch {
		case imm8 < 0x08: return fmt.Sprintf("RLC %s", reg)
		case imm8 < 0x10: return fmt.Sprintf("RRC %s", reg)
		case imm8 < 0x18: return fmt.Sprintf("RL %s", reg)
		case imm8 < 0x20: return fmt.Sprintf("RR %s", reg)
		case imm8 < 0x28: return fmt.Sprintf("SLA %s", reg)
		case imm8 < 0x30: return fmt.Sprintf("SRA %s", reg)
		case imm8 < 0x38: return fmt.Sprintf("SWAP %s", reg)
		case imm8 < 0x40: return fmt.Sprintf("SRL %s", reg)
		case imm8 < 0x80: return fmt.Sprintf("BIT %d,%s", bit, reg)
		case imm8 < 0xC0: return fmt.Sprintf("RES %d,%s", bit, reg)
		default:          return fmt.Sprintf("SET %d,%s", bit, reg)
		}
	case 0xCC: return fmt.Sprintf("CALL Z,%04Xh", imm16)
	case 0xCD: return fmt.Sprintf("CALL %04Xh", imm16)
	case 0xCE: return fmt.Sprintf("ADC A,%02Xh", imm8)
	case 0xCF: return "RST 08h"
	case 0xD0: return "RET NC"
	case 0xD1: return "POP DE"
	case 0xD2: return fmt.Sprintf("JP NC,%04Xh", imm16)
	case 0xD4: return fmt.Sprintf("CALL NC,%04Xh", imm16)
	case 0xD5: return "PUSH DE"
	case 0xD6: return fmt.Sprintf("SUB %02Xh", imm8)
	case 0xD7: return "RST 10h"
	case 0xD8: return "RET C"
	case 0xD9: return "RETI"
	case 0xDA: return fmt.Sprintf("JP C,%04Xh", imm16)
	case 0xDC: return fmt.Sprintf("CALL C,%04Xh", imm16)
	case 0xDE: return fmt.Sprintf("SBC %02Xh", imm8)
	case 0xDF: return "RST 18h"
	case 0xE0: return fmt.Sprintf("LDH (%02Xh),A", imm8)
	case 0xE1: return "POP HL"
	case 0xE2: return "LD (C),A"
	case 0xE5: return "PUSH HL"
	case 0xE6: return fmt.Sprintf("AND %02Xh", imm8)
	case 0xE7: return "RST 20h"
	case 0xE8: return fmt.Sprintf("ADD SP,%d", rel8)
	case 0xE9: return "JP (HL)"
	case 0xEA: return fmt.Sprintf("LD (%04Xh),A", imm16)
	case 0xEE: return fmt.Sprintf("XOR %02Xh", imm8)
	case 0xEF: return "RST 28h"
	case 0xF0: return fmt.Sprintf("LDH A,(%02Xh)", imm8)
	case 0xF1: return "POP AF"
	case 0xF2: return "LD A,(C)"
	case 0xF3: return "DI"
	case 0xF5: return "PUSH AF"
	case 0xF6: return fmt.Sprintf("OR %02Xh", imm8)
	case 0xF7: return "RST 30h"
	case 0xF8: return fmt.Sprintf("LD HL,SP%+d", rel8)
	case 0xF9: return "LD SP,HL"
	case 0xFA: return fmt.Sprintf("LD A,(%04Xh)", imm16)
	case 0xFB: return "EI"
	case 0xFE: return fmt.Sprintf("CP %02Xh", imm8)
	case 0xFF: return "RST 38h"
	}
	return fmt.Sprintf("%02Xh", code)
}

// Returns the time in cycles/4.  One cycle = 1/4194304 seconds.
func (sys *cpu) fdx() int {
	switch sys.fetchByte() {
	case 0x00:		// NOP
		return 4/4
	case 0x01:		// LD BC,d16
		sys.wbc(sys.fetchWord())
		return 12/4
	case 0x02:		// LD (BC),A
		sys.writeByte(sys.bc(), sys.a)
		return 8/4
	case 0x03:		// INC BC
		sys.wbc(sys.bc() + 1)
		return 8/4
	case 0x04:		// INC B
		sys.b = sys.inc(sys.b)
		return 4/4
	case 0x05:		// DEC B
		sys.b = sys.dec(sys.b)
		return 4/4
	case 0x06:		// LD B,d8
		sys.b = sys.fetchByte()
		return 8/4
	case 0x07:		// RLCA
		sys.a = sys.rlc(sys.a)
		return 4/4
	case 0x08:		// LD (a16),SP
		sys.writeWord(sys.fetchWord(), sys.sp)
		return 20/4
	case 0x09:		// ADD HL,BC
		sys.hl = sys.add16(sys.hl, sys.bc())
		return 8/4
	case 0x0A:		// LD A,(BC)
		sys.a = sys.readByte(sys.bc())
		return 8/4
	case 0x0B:		// DEC BC
		sys.wbc(sys.bc() - 1)
		return 8/4
	case 0x0C:		// INC C
		sys.c = sys.inc(sys.c)
		return 4/4
	case 0x0D:		// DEC C
		sys.c = sys.dec(sys.c)
		return 4/4
	case 0x0E:		// LD C,d8
		sys.c = sys.fetchByte()
		return 8/4
	case 0x0F:		// RRCA
		sys.a = sys.rrc(sys.a)
		return 4/4

	case 0x10:		// STOP
		sys.pause = true
		return 4/4
	case 0x11:		// LD DE,d16
		sys.wde(sys.fetchWord())
		return 12/4
	case 0x12:		// LD (DE),A
		sys.writeByte(sys.de(), sys.a)
		return 8/4
	case 0x13:		// INC DE
		sys.wde(sys.de() + 1)
		return 8/4
	case 0x14:		// INC D
		sys.d = sys.inc(sys.d)
		return 4/4
	case 0x15:		// DEC D
		sys.d = sys.dec(sys.d)
		return 4/4
	case 0x16:		// LD D,d8
		sys.d = sys.fetchByte()
		return 8/4
	case 0x17:		// RLA
		sys.a = sys.rl(sys.a)
		return 4/4
	case 0x18:		// JR r8
		return sys.jr(true)
	case 0x19:		// ADD HL,DE
		sys.hl = sys.add16(sys.hl, sys.de())
		return 8/4
	case 0x1A:		// LD A,(DE)
		sys.a = sys.readByte(sys.de())
		return 8/4
	case 0x1B:		// DEC DE
		sys.wde(sys.de() - 1)
		return 8/4
	case 0x1C:		// INC E
		sys.e = sys.inc(sys.e)
		return 4/4
	case 0x1D:		// DEC E
		sys.e = sys.dec(sys.e)
		return 4/4
	case 0x1E:		// LD E,d8
		sys.e = sys.fetchByte()
		return 8/4
	case 0x1F:		// RRA
		sys.a = sys.rr(sys.a)
		return 4/4

	case 0x20:		// JR NZ,r8
		return sys.jr(!sys.fz)
	case 0x21:		// LD HL,d16
		sys.hl = sys.fetchWord()
		return 12/4
	case 0x22:		// LD (HL+),A
		sys.writeByte(sys.hl, sys.a)
		sys.hl++
		return 8/4
	case 0x23:		// INC HL
		sys.hl++
		return 8/4
	case 0x24:		// INC H
		sys.wh(sys.inc(sys.h()))
		return 4/4
	case 0x25:		// DEC H
		sys.wh(sys.dec(sys.h()))
		return 4/4
	case 0x26:		// LD H,d8
		sys.wh(sys.fetchByte())
		return 8/4
	case 0x27:		// DAA
		if sys.fn {
			sys.das()
		} else {
			sys.daa()
		}
		return 4/4
	case 0x28:		// JR Z,r8
		return sys.jr(sys.fz)
	case 0x29:		// ADD HL,HL
		sys.hl = sys.add16(sys.hl, sys.hl)
		return 8/4
	case 0x2A:		// LD A,(HL+)
		sys.a = sys.readByte(sys.hl)
		sys.hl++
		return 8/4
	case 0x2B:		// DEC HL
		sys.hl--
		return 8/4
	case 0x2C:		// INC L
		sys.wl(sys.inc(sys.l()))
		return 4/4
	case 0x2D:		// DEC L
		sys.wl(sys.dec(sys.l()))
		return 4/4
	case 0x2E:		// LD L,d8
		sys.wl(sys.fetchByte())
		return 8/4
	case 0x2F:		// CPL
		sys.a ^= 0xFF
		sys.fn = true
		sys.fh = true
		return 4/4

	case 0x30:		// JR NC,r8
		return sys.jr(!sys.fc)
	case 0x31:		// LD SP,d16
		sys.sp = sys.fetchWord()
		sys.stack = sys.sp
		return 12/4
	case 0x32:		// LD (HL-),A
		sys.writeByte(sys.hl, sys.a)
		sys.hl--
		return 8/4
	case 0x33:		// INC SP
		sys.sp++
		return 8/4
	case 0x34:		// INC (HL)
		x := sys.readByte(sys.hl)
		sys.writeByte(sys.hl, sys.inc(x))
		return 12/4
	case 0x35:		// DEC (HL)
		x := sys.readByte(sys.hl)
		sys.writeByte(sys.hl, sys.dec(x))
		return 12/4
	case 0x36:		// LD (HL),d8
		sys.writeByte(sys.hl, sys.fetchByte())
		return 12/4
	case 0x37:		// SCF
		sys.fn = false
		sys.fh = false
		sys.fc = true
		return 4/4
	case 0x38:		// JR C,r8
		return sys.jr(sys.fc)
	case 0x39:		// ADD HL,SP
		sys.hl = sys.add16(sys.hl, sys.sp)
		return 8/4
	case 0x3A:		// LD A,(HL-)
		sys.a = sys.readByte(sys.hl)
		sys.hl--
		return 8/4
	case 0x3B:		// DEC SP
		sys.sp--
		return 8/4
	case 0x3C:		// INC A
		sys.a = sys.inc(sys.a)
		return 4/4
	case 0x3D:		// DEC A
		sys.a = sys.dec(sys.a)
		return 4/4
	case 0x3E:		// LD A,d8
		sys.a = sys.fetchByte()
		return 8/4
	case 0x3F:		// CCF
		sys.fn = false
		sys.fh = false
		sys.fc = !sys.fc
		return 4/4

	// LD Instructions ///////////////////////////////////////////

	case 0x40: return 1
	case 0x41: sys.b = sys.c; return 1
	case 0x42: sys.b = sys.d; return 1
	case 0x43: sys.b = sys.e; return 1
	case 0x44: sys.b = sys.h(); return 1
	case 0x45: sys.b = sys.l(); return 1
	case 0x46: sys.b = sys.readByte(sys.hl); return 2
	case 0x47: sys.b = sys.a; return 1

	case 0x48: sys.c = sys.b; return 1
	case 0x49: return 1
	case 0x4A: sys.c = sys.d; return 1
	case 0x4B: sys.c = sys.e; return 1
	case 0x4C: sys.c = sys.h(); return 1
	case 0x4D: sys.c = sys.l(); return 1
	case 0x4E: sys.c = sys.readByte(sys.hl); return 2
	case 0x4F: sys.c = sys.a; return 1

	case 0x50: sys.d = sys.b; return 1
	case 0x51: sys.d = sys.c; return 1
	case 0x52: return 1
	case 0x53: sys.d = sys.e; return 1
	case 0x54: sys.d = sys.h(); return 1
	case 0x55: sys.d = sys.l(); return 1
	case 0x56: sys.d = sys.readByte(sys.hl); return 2
	case 0x57: sys.d = sys.a; return 1

	case 0x58: sys.e = sys.b; return 1
	case 0x59: sys.e = sys.c; return 1
	case 0x5A: sys.e = sys.d; return 1
	case 0x5B: return 1
	case 0x5C: sys.e = sys.h(); return 1
	case 0x5D: sys.e = sys.l(); return 1
	case 0x5E: sys.e = sys.readByte(sys.hl); return 2
	case 0x5F: sys.e = sys.a; return 1

	case 0x60: sys.wh(sys.b); return 1
	case 0x61: sys.wh(sys.c); return 1
	case 0x62: sys.wh(sys.d); return 1
	case 0x63: sys.wh(sys.e); return 1
	case 0x64: return 1
	case 0x65: sys.wh(sys.l()); return 1
	case 0x66: sys.wh(sys.readByte(sys.hl)); return 2
	case 0x67: sys.wh(sys.a); return 1

	case 0x68: sys.wl(sys.b); return 1
	case 0x69: sys.wl(sys.c); return 1
	case 0x6A: sys.wl(sys.d); return 1
	case 0x6B: sys.wl(sys.e); return 1
	case 0x6C: sys.wl(sys.h()); return 1
	case 0x6D: return 1
	case 0x6E: sys.wl(sys.readByte(sys.hl)); return 2
	case 0x6F: sys.wl(sys.a); return 1

	case 0x70: sys.writeByte(sys.hl, sys.b); return 2
	case 0x71: sys.writeByte(sys.hl, sys.c); return 2
	case 0x72: sys.writeByte(sys.hl, sys.d); return 2
	case 0x73: sys.writeByte(sys.hl, sys.e); return 2
	case 0x74: sys.writeByte(sys.hl, sys.h()); return 2
	case 0x75: sys.writeByte(sys.hl, sys.l()); return 2
	case 0x76: sys.halt = true; return 1
	case 0x77: sys.writeByte(sys.hl, sys.a); return 2

	case 0x78: sys.a = sys.b; return 1
	case 0x79: sys.a = sys.c; return 1
	case 0x7A: sys.a = sys.d; return 1
	case 0x7B: sys.a = sys.e; return 1
	case 0x7C: sys.a = sys.h(); return 1
	case 0x7D: sys.a = sys.l(); return 1
	case 0x7E: sys.a = sys.readByte(sys.hl); return 2
	case 0x7F: return 1

	// Math Instructions /////////////////////////////////////////

	case 0x80: sys.add(sys.b); return 1
	case 0x81: sys.add(sys.c); return 1
	case 0x82: sys.add(sys.d); return 1
	case 0x83: sys.add(sys.e); return 1
	case 0x84: sys.add(sys.h()); return 1
	case 0x85: sys.add(sys.l()); return 1
	case 0x86: sys.add(sys.readByte(sys.hl)); return 2
	case 0x87: sys.add(sys.a); return 1

	case 0x88: sys.adc(sys.b); return 1
	case 0x89: sys.adc(sys.c); return 1
	case 0x8A: sys.adc(sys.d); return 1
	case 0x8B: sys.adc(sys.e); return 1
	case 0x8C: sys.adc(sys.h()); return 1
	case 0x8D: sys.adc(sys.l()); return 1
	case 0x8E: sys.adc(sys.readByte(sys.hl)); return 2
	case 0x8F: sys.adc(sys.a); return 1

	case 0x90: sys.sub(sys.b); return 1
	case 0x91: sys.sub(sys.c); return 1
	case 0x92: sys.sub(sys.d); return 1
	case 0x93: sys.sub(sys.e); return 1
	case 0x94: sys.sub(sys.h()); return 1
	case 0x95: sys.sub(sys.l()); return 1
	case 0x96: sys.sub(sys.readByte(sys.hl)); return 2
	case 0x97: sys.sub(sys.a); return 1

	case 0x98: sys.sbc(sys.b); return 1
	case 0x99: sys.sbc(sys.c); return 1
	case 0x9A: sys.sbc(sys.d); return 1
	case 0x9B: sys.sbc(sys.e); return 1
	case 0x9C: sys.sbc(sys.h()); return 1
	case 0x9D: sys.sbc(sys.l()); return 1
	case 0x9E: sys.sbc(sys.readByte(sys.hl)); return 2
	case 0x9F: sys.sbc(sys.a); return 1

	case 0xA0: sys.and(sys.b); return 1
	case 0xA1: sys.and(sys.c); return 1
	case 0xA2: sys.and(sys.d); return 1
	case 0xA3: sys.and(sys.e); return 1
	case 0xA4: sys.and(sys.h()); return 1
	case 0xA5: sys.and(sys.l()); return 1
	case 0xA6: sys.and(sys.readByte(sys.hl)); return 2
	case 0xA7: sys.and(sys.a); return 1

	case 0xA8: sys.xor(sys.b); return 1
	case 0xA9: sys.xor(sys.c); return 1
	case 0xAA: sys.xor(sys.d); return 1
	case 0xAB: sys.xor(sys.e); return 1
	case 0xAC: sys.xor(sys.h()); return 1
	case 0xAD: sys.xor(sys.l()); return 1
	case 0xAE: sys.xor(sys.readByte(sys.hl)); return 2
	case 0xAF: sys.xor(sys.a); return 1

	case 0xB0: sys.or(sys.b); return 1
	case 0xB1: sys.or(sys.c); return 1
	case 0xB2: sys.or(sys.d); return 1
	case 0xB3: sys.or(sys.e); return 1
	case 0xB4: sys.or(sys.h()); return 1
	case 0xB5: sys.or(sys.l()); return 1
	case 0xB6: sys.or(sys.readByte(sys.hl)); return 2
	case 0xB7: sys.or(sys.a); return 1

	case 0xB8: sys.cp(sys.b); return 1
	case 0xB9: sys.cp(sys.c); return 1
	case 0xBA: sys.cp(sys.d); return 1
	case 0xBB: sys.cp(sys.e); return 1
	case 0xBC: sys.cp(sys.h()); return 1
	case 0xBD: sys.cp(sys.l()); return 1
	case 0xBE: sys.cp(sys.readByte(sys.hl)); return 2
	case 0xBF: sys.cp(sys.a); return 1

	// Misc Instructions /////////////////////////////////////////

	case 0xC0: 		// RET NZ
		return sys.ret(!sys.fz)
	case 0xC1:		// POP BC
		sys.wbc(sys.pop())
		return 12/4
	case 0xC2:		// JP NZ,a16
		return sys.jp(!sys.fz)
	case 0xC3:		// JP a16
		return sys.jp(true)
	case 0xC4:		// CALL NZ,a16
		return sys.call(!sys.fz)
	case 0xC5:		// PUSH BC
		sys.push(sys.bc())
		return 16/4
	case 0xC6:		// ADD A,d8
		sys.add(sys.fetchByte())
		return 8/4
	case 0xC7:		// RST 00H
		return sys.rst(0x00)
	case 0xC8:		// RET Z
		return sys.ret(sys.fz)
	case 0xC9:		// RET
		sys.pc = sys.pop()
		return 16/4
	case 0xCA:		// JP Z,a16
		return sys.jp(sys.fz)
	case 0xCB:		// ** PREFIX CB **
		return sys.fdxCB()
	case 0xCC:		// CALL Z,a16
		return sys.call(sys.fz)
	case 0xCD:		// CALL a16
		return sys.call(true)
	case 0xCE:		// ADC A,d8
		sys.adc(sys.fetchByte())
		return 8/4
	case 0xCF:		// RST 08H
		return sys.rst(0x08)

	case 0xD0:		// RET NC
		return sys.ret(!sys.fc)
	case 0xD1:		// POP DE
		sys.wde(sys.pop())
		return 12/4
	case 0xD2:		// JP NC,a16
		return sys.jp(!sys.fc)
	case 0xD3: panic("cpu: invalid opcode 0xD3")
	case 0xD4: 		// CALL NC,a16
		return sys.call(!sys.fc)
	case 0xD5:		// PUSH DE
		sys.push(sys.de())
		return 16/4
	case 0xD6:		// SUB d8
		sys.sub(sys.fetchByte())
		return 8/4
	case 0xD7:		// RST 10H
		return sys.rst(0x10)
	case 0xD8:		// RET C
		return sys.ret(sys.fc)
	case 0xD9:		// RETI
		sys.ime = true
		sys.pc = sys.pop()
		return 16/4
	case 0xDA:		// JP C,a16
		return sys.jp(sys.fc)
	case 0xDB: panic("cpu: invalid opcode 0xDB")
	case 0xDC:		// CALL C,a16
		return sys.call(sys.fc)
	case 0xDD: panic("cpu: invalid opcode 0xDD")
	case 0xDE:		// SBC d8
		sys.sbc(sys.fetchByte())
		return 8/4
	case 0xDF:		// RST 18H
		return sys.rst(0x18)

	case 0xE0:		// LDH (a8),A
		addr := 0xFF00 + uint16(sys.fetchByte())
		sys.writePort(addr, sys.a)
		return 12/4
	case 0xE1:		// POP HL
		sys.hl = sys.pop()
		return 12/4
	case 0xE2:		// LD (C),A
		addr := 0xFF00 + uint16(sys.c)
		sys.writePort(addr, sys.a)
		return 8/4
	case 0xE3: panic("cpu: invalid opcode 0xE3")
	case 0xE4: panic("cpu: invalid opcode 0xE4")
	case 0xE5:		// PUSH HL
		sys.push(sys.hl)
		return 16/4
	case 0xE6:		// AND d8
		sys.and(sys.fetchByte())
		return 8/4
	case 0xE7:		// RST 20H
		return sys.rst(0x20)
	case 0xE8:		// ADD SP,r8
		x := sys.fetchByte()
		sys.sp = sys.add16(sys.sp, uint16(int8(x)))
		return 16/4
	case 0xE9:		// JP (HL)
		sys.pc = sys.hl
		return 4/4
	case 0xEA:		// LD (a16),A
		sys.writeByte(sys.fetchWord(), sys.a)
		return 16/4
	case 0xEB: panic("cpu: invalid opcode 0xEB")
	case 0xEC: panic("cpu: invalid opcode 0xEC")
	case 0xED: panic("cpu: invalid opcode 0xED")
	case 0xEE:		// XOR d8
		sys.xor(sys.fetchByte())
		return 8/4
	case 0xEF: 		// RST 28H
		return sys.rst(0x28)

	case 0xF0:		// LDH A,(a8)
		addr := 0xFF00 + uint16(sys.fetchByte())
		sys.a = sys.readPort(addr)
		return 12/4
	case 0xF1:		// POP AF
		sys.waf(sys.pop())
		return 12/4
	case 0xF2:		// LD A,(C)
		addr := 0xFF00 + uint16(sys.c)
		sys.a = sys.readPort(addr)
		return 8/4
	case 0xF3:		// DI
		sys.ime = false
		return 4/4
	case 0xF4: panic("cpu: invalid opcode 0xF4")
	case 0xF5:		// PUSH AF
		sys.push(sys.af())
		return 16/4
	case 0xF6:		// OR d8
		sys.or(sys.fetchByte())
		return 8/4
	case 0xF7:		// RST 30H
		return sys.rst(0x30)
	case 0xF8:		// LD HL,SP+r8
		x := sys.fetchByte()
		sys.hl = sys.add16(sys.sp, uint16(int8(x)))
		return 12/4
	case 0xF9:		// LD SP,HL
		sys.sp = sys.hl
		return 8/4
	case 0xFA:		// LD A,(a16)
		sys.a = sys.readByte(sys.fetchWord())
		return 16/4
	case 0xFB:		// EI
		sys.ime = true
		return 4/4
	case 0xFC: panic("cpu: invalid opcode 0xFC")
	case 0xFD: panic("cpu: invalid opcode 0xFD")
	case 0xFE: 		// CP d8
		sys.cp(sys.fetchByte())
		return 8/4
	case 0xFF:		// RST 38H
		return sys.rst(0x38)
	}
	panic("unreachable in sys.Interpret")
}

func (sys *cpu) fdxCB() int {
	switch sys.fetchByte() {
	case 0x00: sys.b = sys.rlc(sys.b); return 2
	case 0x01: sys.c = sys.rlc(sys.c); return 2
	case 0x02: sys.d = sys.rlc(sys.d); return 2
	case 0x03: sys.e = sys.rlc(sys.e); return 2
	case 0x04: sys.wh(sys.rlc(sys.h())); return 2
	case 0x05: sys.wl(sys.rlc(sys.l())); return 2
	case 0x06: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, sys.rlc(x))
		   return 4
	case 0x07: sys.a = sys.rlc(sys.a); return 2

	case 0x08: sys.b = sys.rrc(sys.b); return 2
	case 0x09: sys.c = sys.rrc(sys.c); return 2
	case 0x0A: sys.d = sys.rrc(sys.d); return 2
	case 0x0B: sys.e = sys.rrc(sys.e); return 2
	case 0x0C: sys.wh(sys.rrc(sys.h())); return 2
	case 0x0D: sys.wl(sys.rrc(sys.l())); return 2
	case 0x0E: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, sys.rrc(x))
		   return 4
	case 0x0F: sys.a = sys.rrc(sys.a); return 2

	case 0x10: sys.b = sys.rl(sys.b); return 2
	case 0x11: sys.c = sys.rl(sys.c); return 2
	case 0x12: sys.d = sys.rl(sys.d); return 2
	case 0x13: sys.e = sys.rl(sys.e); return 2
	case 0x14: sys.wh(sys.rl(sys.h())); return 2
	case 0x15: sys.wl(sys.rl(sys.l())); return 2
	case 0x16: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, sys.rl(x))
		   return 4
	case 0x17: sys.a = sys.rl(sys.a); return 2

	case 0x18: sys.b = sys.rr(sys.b); return 2
	case 0x19: sys.c = sys.rr(sys.c); return 2
	case 0x1A: sys.d = sys.rr(sys.d); return 2
	case 0x1B: sys.e = sys.rr(sys.e); return 2
	case 0x1C: sys.wh(sys.rr(sys.h())); return 2
	case 0x1D: sys.wl(sys.rr(sys.l())); return 2
	case 0x1E: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, sys.rr(x))
		   return 4
	case 0x1F: sys.a = sys.rr(sys.a); return 2

	case 0x20: sys.b = sys.sla(sys.b); return 2
	case 0x21: sys.c = sys.sla(sys.c); return 2
	case 0x22: sys.d = sys.sla(sys.d); return 2
	case 0x23: sys.e = sys.sla(sys.e); return 2
	case 0x24: sys.wh(sys.sla(sys.h())); return 2
	case 0x25: sys.wl(sys.sla(sys.l())); return 2
	case 0x26: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, sys.sla(x))
		   return 4
	case 0x27: sys.a = sys.sla(sys.a); return 2

	case 0x28: sys.b = sys.sra(sys.b); return 2
	case 0x29: sys.c = sys.sra(sys.c); return 2
	case 0x2A: sys.d = sys.sra(sys.d); return 2
	case 0x2B: sys.e = sys.sra(sys.e); return 2
	case 0x2C: sys.wh(sys.sra(sys.h())); return 2
	case 0x2D: sys.wl(sys.sra(sys.l())); return 2
	case 0x2E: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, sys.sra(x))
		   return 4
	case 0x2F: sys.a = sys.sra(sys.a); return 2

	case 0x30: sys.b = sys.swap(sys.b); return 2
	case 0x31: sys.c = sys.swap(sys.c); return 2
	case 0x32: sys.d = sys.swap(sys.d); return 2
	case 0x33: sys.e = sys.swap(sys.e); return 2
	case 0x34: sys.wh(sys.swap(sys.h())); return 2
	case 0x35: sys.wl(sys.swap(sys.l())); return 2
	case 0x36: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, sys.swap(x))
		   return 4
	case 0x37: sys.a = sys.swap(sys.a); return 2

	case 0x38: sys.b = sys.srl(sys.b); return 2
	case 0x39: sys.c = sys.srl(sys.c); return 2
	case 0x3A: sys.d = sys.srl(sys.d); return 2
	case 0x3B: sys.e = sys.srl(sys.e); return 2
	case 0x3C: sys.wh(sys.srl(sys.h())); return 2
	case 0x3D: sys.wl(sys.srl(sys.l())); return 2
	case 0x3E: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, sys.srl(x))
		   return 4
	case 0x3F: sys.a = sys.srl(sys.a); return 2

	case 0x40: sys.bit(0, sys.b); return 2
	case 0x41: sys.bit(0, sys.c); return 2
	case 0x42: sys.bit(0, sys.d); return 2
	case 0x43: sys.bit(0, sys.e); return 2
	case 0x44: sys.bit(0, sys.h()); return 2
	case 0x45: sys.bit(0, sys.l()); return 2
	case 0x46: sys.bit(0, sys.readByte(sys.hl)); return 4
	case 0x47: sys.bit(0, sys.a); return 2

	case 0x48: sys.bit(1, sys.b); return 2
	case 0x49: sys.bit(1, sys.c); return 2
	case 0x4A: sys.bit(1, sys.d); return 2
	case 0x4B: sys.bit(1, sys.e); return 2
	case 0x4C: sys.bit(1, sys.h()); return 2
	case 0x4D: sys.bit(1, sys.l()); return 2
	case 0x4E: sys.bit(1, sys.readByte(sys.hl)); return 4
	case 0x4F: sys.bit(1, sys.a); return 2

	case 0x50: sys.bit(2, sys.b); return 2
	case 0x51: sys.bit(2, sys.c); return 2
	case 0x52: sys.bit(2, sys.d); return 2
	case 0x53: sys.bit(2, sys.e); return 2
	case 0x54: sys.bit(2, sys.h()); return 2
	case 0x55: sys.bit(2, sys.l()); return 2
	case 0x56: sys.bit(2, sys.readByte(sys.hl)); return 4
	case 0x57: sys.bit(2, sys.a); return 2

	case 0x58: sys.bit(3, sys.b); return 2
	case 0x59: sys.bit(3, sys.c); return 2
	case 0x5A: sys.bit(3, sys.d); return 2
	case 0x5B: sys.bit(3, sys.e); return 2
	case 0x5C: sys.bit(3, sys.h()); return 2
	case 0x5D: sys.bit(3, sys.l()); return 2
	case 0x5E: sys.bit(3, sys.readByte(sys.hl)); return 4
	case 0x5F: sys.bit(3, sys.a); return 2

	case 0x60: sys.bit(4, sys.b); return 2
	case 0x61: sys.bit(4, sys.c); return 2
	case 0x62: sys.bit(4, sys.d); return 2
	case 0x63: sys.bit(4, sys.e); return 2
	case 0x64: sys.bit(4, sys.h()); return 2
	case 0x65: sys.bit(4, sys.l()); return 2
	case 0x66: sys.bit(4, sys.readByte(sys.hl)); return 4
	case 0x67: sys.bit(4, sys.a); return 2

	case 0x68: sys.bit(5, sys.b); return 2
	case 0x69: sys.bit(5, sys.c); return 2
	case 0x6A: sys.bit(5, sys.d); return 2
	case 0x6B: sys.bit(5, sys.e); return 2
	case 0x6C: sys.bit(5, sys.h()); return 2
	case 0x6D: sys.bit(5, sys.l()); return 2
	case 0x6E: sys.bit(5, sys.readByte(sys.hl)); return 4
	case 0x6F: sys.bit(5, sys.a); return 2

	case 0x70: sys.bit(6, sys.b); return 2
	case 0x71: sys.bit(6, sys.c); return 2
	case 0x72: sys.bit(6, sys.d); return 2
	case 0x73: sys.bit(6, sys.e); return 2
	case 0x74: sys.bit(6, sys.h()); return 2
	case 0x75: sys.bit(6, sys.l()); return 2
	case 0x76: sys.bit(6, sys.readByte(sys.hl)); return 4
	case 0x77: sys.bit(6, sys.a); return 2

	case 0x78: sys.bit(7, sys.b); return 2
	case 0x79: sys.bit(7, sys.c); return 2
	case 0x7A: sys.bit(7, sys.d); return 2
	case 0x7B: sys.bit(7, sys.e); return 2
	case 0x7C: sys.bit(7, sys.h()); return 2
	case 0x7D: sys.bit(7, sys.l()); return 2
	case 0x7E: sys.bit(7, sys.readByte(sys.hl)); return 4
	case 0x7F: sys.bit(7, sys.a); return 2

	case 0x80: sys.b = res(0, sys.b); return 2
	case 0x81: sys.c = res(0, sys.c); return 2
	case 0x82: sys.d = res(0, sys.d); return 2
	case 0x83: sys.e = res(0, sys.e); return 2
	case 0x84: sys.wh(res(0, sys.h())); return 2
	case 0x85: sys.wl(res(0, sys.l())); return 2
	case 0x86: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, res(0, x))
		   return 4
	case 0x87: sys.a = res(0, sys.a); return 2

	case 0x88: sys.b = res(1, sys.b); return 2
	case 0x89: sys.c = res(1, sys.c); return 2
	case 0x8A: sys.d = res(1, sys.d); return 2
	case 0x8B: sys.e = res(1, sys.e); return 2
	case 0x8C: sys.wh(res(1, sys.h())); return 2
	case 0x8D: sys.wl(res(1, sys.l())); return 2
	case 0x8E: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, res(1, x))
		   return 4
	case 0x8F: sys.a = res(1, sys.a); return 2

	case 0x90: sys.b = res(2, sys.b); return 2
	case 0x91: sys.c = res(2, sys.c); return 2
	case 0x92: sys.d = res(2, sys.d); return 2
	case 0x93: sys.e = res(2, sys.e); return 2
	case 0x94: sys.wh(res(2, sys.h())); return 2
	case 0x95: sys.wl(res(2, sys.l())); return 2
	case 0x96: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, res(2, x))
		   return 4
	case 0x97: sys.a = res(2, sys.a); return 2

	case 0x98: sys.b = res(3, sys.b); return 2
	case 0x99: sys.c = res(3, sys.c); return 2
	case 0x9A: sys.d = res(3, sys.d); return 2
	case 0x9B: sys.e = res(3, sys.e); return 2
	case 0x9C: sys.wh(res(3, sys.h())); return 2
	case 0x9D: sys.wl(res(3, sys.l())); return 2
	case 0x9E: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, res(3, x))
		   return 4
	case 0x9F: sys.a = res(3, sys.a); return 2

	case 0xA0: sys.b = res(4, sys.b); return 2
	case 0xA1: sys.c = res(4, sys.c); return 2
	case 0xA2: sys.d = res(4, sys.d); return 2
	case 0xA3: sys.e = res(4, sys.e); return 2
	case 0xA4: sys.wh(res(4, sys.h())); return 2
	case 0xA5: sys.wl(res(4, sys.l())); return 2
	case 0xA6: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, res(4, x))
		   return 4
	case 0xA7: sys.a = res(4, sys.a); return 2

	case 0xA8: sys.b = res(5, sys.b); return 2
	case 0xA9: sys.c = res(5, sys.c); return 2
	case 0xAA: sys.d = res(5, sys.d); return 2
	case 0xAB: sys.e = res(5, sys.e); return 2
	case 0xAC: sys.wh(res(5, sys.h())); return 2
	case 0xAD: sys.wl(res(5, sys.l())); return 2
	case 0xAE: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, res(5, x))
		   return 4
	case 0xAF: sys.a = res(5, sys.a); return 2

	case 0xB0: sys.b = res(6, sys.b); return 2
	case 0xB1: sys.c = res(6, sys.c); return 2
	case 0xB2: sys.d = res(6, sys.d); return 2
	case 0xB3: sys.e = res(6, sys.e); return 2
	case 0xB4: sys.wh(res(6, sys.h())); return 2
	case 0xB5: sys.wl(res(6, sys.l())); return 2
	case 0xB6: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, res(6, x))
		   return 4
	case 0xB7: sys.a = res(6, sys.a); return 2

	case 0xB8: sys.b = res(7, sys.b); return 2
	case 0xB9: sys.c = res(7, sys.c); return 2
	case 0xBA: sys.d = res(7, sys.d); return 2
	case 0xBB: sys.e = res(7, sys.e); return 2
	case 0xBC: sys.wh(res(7, sys.h())); return 2
	case 0xBD: sys.wl(res(7, sys.l())); return 2
	case 0xBE: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, res(7, x))
		   return 4
	case 0xBF: sys.a = res(7, sys.a); return 2

	case 0xC0: sys.b = set(0, sys.b); return 2
	case 0xC1: sys.c = set(0, sys.c); return 2
	case 0xC2: sys.d = set(0, sys.d); return 2
	case 0xC3: sys.e = set(0, sys.e); return 2
	case 0xC4: sys.wh(set(0, sys.h())); return 2
	case 0xC5: sys.wl(set(0, sys.l())); return 2
	case 0xC6: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, set(0, x))
		   return 4
	case 0xC7: sys.a = set(0, sys.a); return 2

	case 0xC8: sys.b = set(1, sys.b); return 2
	case 0xC9: sys.c = set(1, sys.c); return 2
	case 0xCA: sys.d = set(1, sys.d); return 2
	case 0xCB: sys.e = set(1, sys.e); return 2
	case 0xCC: sys.wh(set(1, sys.h())); return 2
	case 0xCD: sys.wl(set(1, sys.l())); return 2
	case 0xCE: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, set(1, x))
		   return 4
	case 0xCF: sys.a = set(1, sys.a); return 2

	case 0xD0: sys.b = set(2, sys.b); return 2
	case 0xD1: sys.c = set(2, sys.c); return 2
	case 0xD2: sys.d = set(2, sys.d); return 2
	case 0xD3: sys.e = set(2, sys.e); return 2
	case 0xD4: sys.wh(set(2, sys.h())); return 2
	case 0xD5: sys.wl(set(2, sys.l())); return 2
	case 0xD6: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, set(2, x))
		   return 4
	case 0xD7: sys.a = set(2, sys.a); return 2

	case 0xD8: sys.b = set(3, sys.b); return 2
	case 0xD9: sys.c = set(3, sys.c); return 2
	case 0xDA: sys.d = set(3, sys.d); return 2
	case 0xDB: sys.e = set(3, sys.e); return 2
	case 0xDC: sys.wh(set(3, sys.h())); return 2
	case 0xDD: sys.wl(set(3, sys.l())); return 2
	case 0xDE: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, set(3, x))
		   return 4
	case 0xDF: sys.a = set(3, sys.a); return 2

	case 0xE0: sys.b = set(4, sys.b); return 2
	case 0xE1: sys.c = set(4, sys.c); return 2
	case 0xE2: sys.d = set(4, sys.d); return 2
	case 0xE3: sys.e = set(4, sys.e); return 2
	case 0xE4: sys.wh(set(4, sys.h())); return 2
	case 0xE5: sys.wl(set(4, sys.l())); return 2
	case 0xE6: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, set(4, x))
		   return 4
	case 0xE7: sys.a = set(4, sys.a); return 2

	case 0xE8: sys.b = set(5, sys.b); return 2
	case 0xE9: sys.c = set(5, sys.c); return 2
	case 0xEA: sys.d = set(5, sys.d); return 2
	case 0xEB: sys.e = set(5, sys.e); return 2
	case 0xEC: sys.wh(set(5, sys.h())); return 2
	case 0xED: sys.wl(set(5, sys.l())); return 2
	case 0xEE: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, set(5, x))
		   return 4
	case 0xEF: sys.a = set(5, sys.a); return 2

	case 0xF0: sys.b = set(6, sys.b); return 2
	case 0xF1: sys.c = set(6, sys.c); return 2
	case 0xF2: sys.d = set(6, sys.d); return 2
	case 0xF3: sys.e = set(6, sys.e); return 2
	case 0xF4: sys.wh(set(6, sys.h())); return 2
	case 0xF5: sys.wl(set(6, sys.l())); return 2
	case 0xF6: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, set(6, x))
		   return 4
	case 0xF7: sys.a = set(6, sys.a); return 2

	case 0xF8: sys.b = set(7, sys.b); return 2
	case 0xF9: sys.c = set(7, sys.c); return 2
	case 0xFA: sys.d = set(7, sys.d); return 2
	case 0xFB: sys.e = set(7, sys.e); return 2
	case 0xFC: sys.wh(set(7, sys.h())); return 2
	case 0xFD: sys.wl(set(7, sys.l())); return 2
	case 0xFE: x := sys.readByte(sys.hl)
		   sys.writeByte(sys.hl, set(7, x))
		   return 4
	case 0xFF: sys.a = set(7, sys.a); return 2
	}
	panic("unreachable in sys.Interpret (CB)")
}

func (sys *cpu) jr(pred bool) int {
	x := sys.fetchByte()
	if pred {
		sys.pc += uint16(int8(x))
		return 12/4
	}
	return 8/4
}

func (sys *cpu) ret(pred bool) int {
	if pred {
		sys.pc = sys.pop()
		return 20/4
	}
	return 8/4
}

func (sys *cpu) jp(pred bool) int {
	x := sys.fetchWord()
	if pred {
		sys.pc = x
		return 16/4
	}
	return 12/4
}

func (sys *cpu) call(pred bool) int {
	x := sys.fetchWord()
	if pred {
		sys.push(sys.pc)
		sys.pc = x
		return 24/4
	}
	return 12/4
}

func (sys *cpu) rst(addr uint16) int {
	sys.push(sys.pc)
	sys.pc = addr
	return 32/4
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
	sys.fh = x1 & 0x0FFF < x & 0x0FFF
	sys.fc = x1 < x
	return x1
}

func (sys *cpu) add(x byte) {
	y := sys.a + x
	sys.fz = y == 0
	sys.fn = false
	sys.fh = y & 0x0F < sys.a & 0x0F
	sys.fc = y < sys.a
	sys.a = y
}

func (sys *cpu) adc(x byte) {
	fc := byte(0)
	if sys.fc { fc = 1 }
	y := sys.a + x + fc
	sys.fz = y == 0
	sys.fn = false
	sys.fh = y & 0x0F < sys.a & 0x0F
	sys.fc = y < sys.a
	sys.a = y
}

func (sys *cpu) sub(x byte) {
	y := sys.a - x
	sys.fz = y == 0
	sys.fn = true
	sys.fh = y & 0x0F > sys.a & 0x0F
	sys.fc = y > sys.a
	sys.a = y
}

func (sys *cpu) sbc(x byte) {
	fc := byte(0)
	if sys.fc { fc = 1 }
	y := sys.a - x - fc
	sys.fz = y == 0
	sys.fn = true
	sys.fh = y & 0x0F > sys.a & 0x0F
	sys.fc = y > sys.a
	sys.a = y
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
	sys.fh = y & 0x0F > sys.a & 0x0F
	sys.fc = y > sys.a
}

func (sys *cpu) rlc(x byte) byte {
	fc := x >> 7
	y := x << 1 | fc
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = fc == 1
	return y
}

func (sys *cpu) rrc(x byte) byte {
	fc := x << 7
	y := x >> 1 | fc
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = fc == 0x80
	return y
}

func (sys *cpu) rl(x byte) byte {
	fc := byte(0)
	if sys.fc { fc = 1 }
	y := x << 1 | fc
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = x & 1 == 1
	return y
}

func (sys *cpu) rr(x byte) byte {
	fc := byte(0)
	if sys.fc { fc = 0x80 }
	y := x >> 1 | fc
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = x & 0x80 == 0x80
	return y
}

func (sys *cpu) sla(x byte) byte {
	y := x << 1
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = x & 0x80 == 0x80
	return y
}

func (sys *cpu) sra(x byte) byte {
	y := byte(int8(x) >> 1)
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = x & 1 == 1
	return y
}

func (sys *cpu) srl(x byte) byte {
	y := x >> 1
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = x & 1 == 1
	return y
}

func (sys *cpu) swap(x byte) byte {
	y := x >> 4 | x << 4
	sys.fz = y == 0
	sys.fn = false
	sys.fh = false
	sys.fc = false
	return y
}

func (sys *cpu) bit(n, x byte) {
	sys.fz = x & (1 << n) != 0
	sys.fn = false
	sys.fh = true
}

func res(n, x byte) byte {
	return x &^ (1 << n)
}

func set(n, x byte) byte {
	return x | 1 << n
}

// DAA and DAS implementations based on pseudocode from 80386
// instruction set references.

func (sys *cpu) daa() {
	if sys.a & 0x0F > 9 || sys.fh {
		sys.a += 6
		sys.fh = true
	} else {
		sys.fh = false
	}
	if sys.a > 0x9F || sys.fc {
		sys.a += 0x60
		sys.fc = true
	} else {
		sys.fc = false
	}
	sys.fz = sys.a == 0
}

func (sys *cpu) das() {
	if sys.a & 0x0F > 9 || sys.fh {
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
	if sys.fz { f |= 0x80 }
	if sys.fn { f |= 0x40 }
	if sys.fh { f |= 0x20 }
	if sys.fc { f |= 0x10 }
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
