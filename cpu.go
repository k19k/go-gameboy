package main

import (
	"fmt"
)

type CPU struct {
	a, b, c, d, e byte
	hl, pc, sp uint16
	fz, fn, fh, fc bool
	ime, halt, pause bool
	mmu *MBC
	PC uint16
}

func NewCPU(mmu *MBC) *CPU {
	return &CPU { a: 0x01, b: 0x00, c: 0x13, d: 0x00, e: 0xD8,
		hl: 0x014D, pc: 0x0100, sp: 0xFFFE,
		fz: true, fn: false, fh: true, fc: true,
		ime: true, halt: false, pause: true,
		mmu: mmu }
		
}

func (cpu *CPU) String() string {
	return fmt.Sprintf(
		"<CPU AF=%04X BC=%04X DE=%04X HL=%04X\n" +
		"     PC=%04X SP=%04X\n" +
		"     IME=%t Halt=%t Pause=%t>",
		cpu.af(), cpu.bc(), cpu.de(), cpu.hl,
		cpu.pc, cpu.sp, cpu.ime, cpu.halt, cpu.pause)
}

func (cpu *CPU) Step() int {
	t := 4
	if !cpu.halt {
		cpu.PC = cpu.pc
		//fmt.Printf("%04X %s\n", cpu.pc, cpu.mmu.Disasm(cpu.pc))
		t = cpu.fdx()
	}
	if cpu.ime {
		f := cpu.mmu.ReadPort(PortIF)
		e := cpu.mmu.ReadPort(PortIE)
		mask := f & e & 0x1F
		if mask != 0 {
			t += cpu.irq(mask, f)
		}
	}
	return t
}

func (cpu *CPU) irq(mask, f byte) int {
	cpu.ime = false
	cpu.halt = false
	cpu.push(cpu.pc)
	if mask & 0x01 != 0 {
		cpu.pc = VBlankAddr
		f &^= 0x01
	} else if mask & 0x02 != 0 {
		cpu.pc = LCDStatusAddr
		f &^= 0x02
	} else if mask & 0x04 != 0 {
		cpu.pc = TimerAddr
		f &^= 0x04
	} else if mask & 0x08 != 0 {
		cpu.pc = SerialAddr
		f &^= 0x08
	} else if mask & 0x10 != 0 {
		cpu.pc = JoypadAddr
		f &^= 0x10
	}
	cpu.mmu.WritePort(PortIF, f)
	return 32/4
}

func (mmu *MBC) Disasm(addr uint16) string {
	code := mmu.ReadByte(addr)
	imm8 := mmu.ReadByte(addr+1)
	imm16 := mmu.ReadWord(addr+1)
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
func (cpu *CPU) fdx() int {
	switch cpu.fetchByte() {
	case 0x00:		// NOP
		return 4/4
	case 0x01:		// LD BC,d16
		cpu.wbc(cpu.fetchWord())
		return 12/4
	case 0x02:		// LD (BC),A
		cpu.mmu.WriteByte(cpu.bc(), cpu.a)
		return 8/4
	case 0x03:		// INC BC
		cpu.wbc(cpu.bc() + 1)
		return 8/4
	case 0x04:		// INC B
		cpu.b = cpu.inc(cpu.b)
		return 4/4
	case 0x05:		// DEC B
		cpu.b = cpu.dec(cpu.b)
		return 4/4
	case 0x06:		// LD B,d8
		cpu.b = cpu.fetchByte()
		return 8/4
	case 0x07:		// RLCA
		cpu.a = cpu.rlc(cpu.a)
		return 4/4
	case 0x08:		// LD (a16),SP
		cpu.mmu.WriteWord(cpu.fetchWord(), cpu.sp)
		return 20/4
	case 0x09:		// ADD HL,BC
		cpu.hl = cpu.add16(cpu.hl, cpu.bc())
		return 8/4
	case 0x0A:		// LD A,(BC)
		cpu.a = cpu.mmu.ReadByte(cpu.bc())
		return 8/4
	case 0x0B:		// DEC BC
		cpu.wbc(cpu.bc() - 1)
		return 8/4
	case 0x0C:		// INC C
		cpu.c = cpu.inc(cpu.c)
		return 4/4
	case 0x0D:		// DEC C
		cpu.c = cpu.dec(cpu.c)
		return 4/4
	case 0x0E:		// LD C,d8
		cpu.c = cpu.fetchByte()
		return 8/4
	case 0x0F:		// RRCA
		cpu.a = cpu.rrc(cpu.a)
		return 4/4

	case 0x10:		// STOP
		cpu.pause = true
		return 4/4
	case 0x11:		// LD DE,d16
		cpu.wde(cpu.fetchWord())
		return 12/4
	case 0x12:		// LD (DE),A
		cpu.mmu.WriteByte(cpu.de(), cpu.a)
		return 8/4
	case 0x13:		// INC DE
		cpu.wde(cpu.de() + 1)
		return 8/4
	case 0x14:		// INC D
		cpu.d = cpu.inc(cpu.d)
		return 4/4
	case 0x15:		// DEC D
		cpu.d = cpu.dec(cpu.d)
		return 4/4
	case 0x16:		// LD D,d8
		cpu.d = cpu.fetchByte()
		return 8/4
	case 0x17:		// RLA
		cpu.a = cpu.rl(cpu.a)
		return 4/4
	case 0x18:		// JR r8
		return cpu.jr(true)
	case 0x19:		// ADD HL,DE
		cpu.hl = cpu.add16(cpu.hl, cpu.de())
		return 8/4
	case 0x1A:		// LD A,(DE)
		cpu.a = cpu.mmu.ReadByte(cpu.de())
		return 8/4
	case 0x1B:		// DEC DE
		cpu.wde(cpu.de() - 1)
		return 8/4
	case 0x1C:		// INC E
		cpu.e = cpu.inc(cpu.e)
		return 4/4
	case 0x1D:		// DEC E
		cpu.e = cpu.dec(cpu.e)
		return 4/4
	case 0x1E:		// LD E,d8
		cpu.e = cpu.fetchByte()
		return 8/4
	case 0x1F:		// RRA
		cpu.a = cpu.rr(cpu.a)
		return 4/4

	case 0x20:		// JR NZ,r8
		return cpu.jr(!cpu.fz)
	case 0x21:		// LD HL,d16
		cpu.hl = cpu.fetchWord()
		return 12/4
	case 0x22:		// LD (HL+),A
		cpu.mmu.WriteByte(cpu.hl, cpu.a)
		cpu.hl++
		return 8/4
	case 0x23:		// INC HL
		cpu.hl++
		return 8/4
	case 0x24:		// INC H
		cpu.wh(cpu.inc(cpu.h()))
		return 4/4
	case 0x25:		// DEC H
		cpu.wh(cpu.dec(cpu.h()))
		return 4/4
	case 0x26:		// LD H,d8
		cpu.wh(cpu.fetchByte())
		return 8/4
	case 0x27:		// DAA
		if cpu.fn {
			cpu.das()
		} else {
			cpu.daa()
		}
		return 4/4
	case 0x28:		// JR Z,r8
		return cpu.jr(cpu.fz)
	case 0x29:		// ADD HL,HL
		cpu.hl = cpu.add16(cpu.hl, cpu.hl)
		return 8/4
	case 0x2A:		// LD A,(HL+)
		cpu.a = cpu.mmu.ReadByte(cpu.hl)
		cpu.hl++
		return 8/4
	case 0x2B:		// DEC HL
		cpu.hl--
		return 8/4
	case 0x2C:		// INC L
		cpu.wl(cpu.inc(cpu.l()))
		return 4/4
	case 0x2D:		// DEC L
		cpu.wl(cpu.dec(cpu.l()))
		return 4/4
	case 0x2E:		// LD L,d8
		cpu.wl(cpu.fetchByte())
		return 8/4
	case 0x2F:		// CPL
		cpu.a ^= 0xFF
		cpu.fn = true
		cpu.fh = true
		return 4/4

	case 0x30:		// JR NC,r8
		return cpu.jr(!cpu.fc)
	case 0x31:		// LD SP,d16
		cpu.sp = cpu.fetchWord()
		return 12/4
	case 0x32:		// LD (HL-),A
		cpu.mmu.WriteByte(cpu.hl, cpu.a)
		cpu.hl--
		return 8/4
	case 0x33:		// INC SP
		cpu.sp++
		return 8/4
	case 0x34:		// INC (HL)
		x := cpu.mmu.ReadByte(cpu.hl)
		cpu.mmu.WriteByte(cpu.hl, cpu.inc(x))
		return 12/4
	case 0x35:		// DEC (HL)
		x := cpu.mmu.ReadByte(cpu.hl)
		cpu.mmu.WriteByte(cpu.hl, cpu.dec(x))
		return 12/4
	case 0x36:		// LD (HL),d8
		cpu.mmu.WriteByte(cpu.hl, cpu.fetchByte())
		return 12/4
	case 0x37:		// SCF
		cpu.fn = false
		cpu.fh = false
		cpu.fc = true
		return 4/4
	case 0x38:		// JR C,r8
		return cpu.jr(cpu.fc)
	case 0x39:		// ADD HL,SP
		cpu.hl = cpu.add16(cpu.hl, cpu.sp)
		return 8/4
	case 0x3A:		// LD A,(HL-)
		cpu.a = cpu.mmu.ReadByte(cpu.hl)
		cpu.hl--
		return 8/4
	case 0x3B:		// DEC SP
		cpu.sp--
		return 8/4
	case 0x3C:		// INC A
		cpu.a = cpu.inc(cpu.a)
		return 4/4
	case 0x3D:		// DEC A
		cpu.a = cpu.dec(cpu.a)
		return 4/4
	case 0x3E:		// LD A,d8
		cpu.a = cpu.fetchByte()
		return 8/4
	case 0x3F:		// CCF
		cpu.fn = false
		cpu.fh = false
		cpu.fc = !cpu.fc
		return 4/4

	// LD Instructions ///////////////////////////////////////////

	case 0x40: return 1
	case 0x41: cpu.b = cpu.c; return 1
	case 0x42: cpu.b = cpu.d; return 1
	case 0x43: cpu.b = cpu.e; return 1
	case 0x44: cpu.b = cpu.h(); return 1
	case 0x45: cpu.b = cpu.l(); return 1
	case 0x46: cpu.b = cpu.mmu.ReadByte(cpu.hl); return 2
	case 0x47: cpu.b = cpu.a; return 1

	case 0x48: cpu.c = cpu.b; return 1
	case 0x49: return 1
	case 0x4A: cpu.c = cpu.d; return 1
	case 0x4B: cpu.c = cpu.e; return 1
	case 0x4C: cpu.c = cpu.h(); return 1
	case 0x4D: cpu.c = cpu.l(); return 1
	case 0x4E: cpu.c = cpu.mmu.ReadByte(cpu.hl); return 2
	case 0x4F: cpu.c = cpu.a; return 1

	case 0x50: cpu.d = cpu.b; return 1
	case 0x51: cpu.d = cpu.c; return 1
	case 0x52: return 1
	case 0x53: cpu.d = cpu.e; return 1
	case 0x54: cpu.d = cpu.h(); return 1
	case 0x55: cpu.d = cpu.l(); return 1
	case 0x56: cpu.d = cpu.mmu.ReadByte(cpu.hl); return 2
	case 0x57: cpu.d = cpu.a; return 1

	case 0x58: cpu.e = cpu.b; return 1
	case 0x59: cpu.e = cpu.c; return 1
	case 0x5A: cpu.e = cpu.d; return 1
	case 0x5B: return 1
	case 0x5C: cpu.e = cpu.h(); return 1
	case 0x5D: cpu.e = cpu.l(); return 1
	case 0x5E: cpu.e = cpu.mmu.ReadByte(cpu.hl); return 2
	case 0x5F: cpu.e = cpu.a; return 1

	case 0x60: cpu.wh(cpu.b); return 1
	case 0x61: cpu.wh(cpu.c); return 1
	case 0x62: cpu.wh(cpu.d); return 1
	case 0x63: cpu.wh(cpu.e); return 1
	case 0x64: return 1
	case 0x65: cpu.wh(cpu.l()); return 1
	case 0x66: cpu.wh(cpu.mmu.ReadByte(cpu.hl)); return 2
	case 0x67: cpu.wh(cpu.a); return 1

	case 0x68: cpu.wl(cpu.b); return 1
	case 0x69: cpu.wl(cpu.c); return 1
	case 0x6A: cpu.wl(cpu.d); return 1
	case 0x6B: cpu.wl(cpu.e); return 1
	case 0x6C: cpu.wl(cpu.h()); return 1
	case 0x6D: return 1
	case 0x6E: cpu.wl(cpu.mmu.ReadByte(cpu.hl)); return 2
	case 0x6F: cpu.wl(cpu.a); return 1

	case 0x70: cpu.mmu.WriteByte(cpu.hl, cpu.b); return 2
	case 0x71: cpu.mmu.WriteByte(cpu.hl, cpu.c); return 2
	case 0x72: cpu.mmu.WriteByte(cpu.hl, cpu.d); return 2
	case 0x73: cpu.mmu.WriteByte(cpu.hl, cpu.e); return 2
	case 0x74: cpu.mmu.WriteByte(cpu.hl, cpu.h()); return 2
	case 0x75: cpu.mmu.WriteByte(cpu.hl, cpu.l()); return 2
	case 0x76: cpu.halt = true; return 1
	case 0x77: cpu.mmu.WriteByte(cpu.hl, cpu.a); return 2

	case 0x78: cpu.a = cpu.b; return 1
	case 0x79: cpu.a = cpu.c; return 1
	case 0x7A: cpu.a = cpu.d; return 1
	case 0x7B: cpu.a = cpu.e; return 1
	case 0x7C: cpu.a = cpu.h(); return 1
	case 0x7D: cpu.a = cpu.l(); return 1
	case 0x7E: cpu.a = cpu.mmu.ReadByte(cpu.hl); return 2
	case 0x7F: return 1

	// Math Instructions /////////////////////////////////////////

	case 0x80: cpu.add(cpu.b); return 1
	case 0x81: cpu.add(cpu.c); return 1
	case 0x82: cpu.add(cpu.d); return 1
	case 0x83: cpu.add(cpu.e); return 1
	case 0x84: cpu.add(cpu.h()); return 1
	case 0x85: cpu.add(cpu.l()); return 1
	case 0x86: cpu.add(cpu.mmu.ReadByte(cpu.hl)); return 2
	case 0x87: cpu.add(cpu.a); return 1

	case 0x88: cpu.adc(cpu.b); return 1
	case 0x89: cpu.adc(cpu.c); return 1
	case 0x8A: cpu.adc(cpu.d); return 1
	case 0x8B: cpu.adc(cpu.e); return 1
	case 0x8C: cpu.adc(cpu.h()); return 1
	case 0x8D: cpu.adc(cpu.l()); return 1
	case 0x8E: cpu.adc(cpu.mmu.ReadByte(cpu.hl)); return 2
	case 0x8F: cpu.adc(cpu.a); return 1

	case 0x90: cpu.sub(cpu.b); return 1
	case 0x91: cpu.sub(cpu.c); return 1
	case 0x92: cpu.sub(cpu.d); return 1
	case 0x93: cpu.sub(cpu.e); return 1
	case 0x94: cpu.sub(cpu.h()); return 1
	case 0x95: cpu.sub(cpu.l()); return 1
	case 0x96: cpu.sub(cpu.mmu.ReadByte(cpu.hl)); return 2
	case 0x97: cpu.sub(cpu.a); return 1

	case 0x98: cpu.sbc(cpu.b); return 1
	case 0x99: cpu.sbc(cpu.c); return 1
	case 0x9A: cpu.sbc(cpu.d); return 1
	case 0x9B: cpu.sbc(cpu.e); return 1
	case 0x9C: cpu.sbc(cpu.h()); return 1
	case 0x9D: cpu.sbc(cpu.l()); return 1
	case 0x9E: cpu.sbc(cpu.mmu.ReadByte(cpu.hl)); return 2
	case 0x9F: cpu.sbc(cpu.a); return 1

	case 0xA0: cpu.and(cpu.b); return 1
	case 0xA1: cpu.and(cpu.c); return 1
	case 0xA2: cpu.and(cpu.d); return 1
	case 0xA3: cpu.and(cpu.e); return 1
	case 0xA4: cpu.and(cpu.h()); return 1
	case 0xA5: cpu.and(cpu.l()); return 1
	case 0xA6: cpu.and(cpu.mmu.ReadByte(cpu.hl)); return 2
	case 0xA7: cpu.and(cpu.a); return 1

	case 0xA8: cpu.xor(cpu.b); return 1
	case 0xA9: cpu.xor(cpu.c); return 1
	case 0xAA: cpu.xor(cpu.d); return 1
	case 0xAB: cpu.xor(cpu.e); return 1
	case 0xAC: cpu.xor(cpu.h()); return 1
	case 0xAD: cpu.xor(cpu.l()); return 1
	case 0xAE: cpu.xor(cpu.mmu.ReadByte(cpu.hl)); return 2
	case 0xAF: cpu.xor(cpu.a); return 1

	case 0xB0: cpu.or(cpu.b); return 1
	case 0xB1: cpu.or(cpu.c); return 1
	case 0xB2: cpu.or(cpu.d); return 1
	case 0xB3: cpu.or(cpu.e); return 1
	case 0xB4: cpu.or(cpu.h()); return 1
	case 0xB5: cpu.or(cpu.l()); return 1
	case 0xB6: cpu.or(cpu.mmu.ReadByte(cpu.hl)); return 2
	case 0xB7: cpu.or(cpu.a); return 1

	case 0xB8: cpu.cp(cpu.b); return 1
	case 0xB9: cpu.cp(cpu.c); return 1
	case 0xBA: cpu.cp(cpu.d); return 1
	case 0xBB: cpu.cp(cpu.e); return 1
	case 0xBC: cpu.cp(cpu.h()); return 1
	case 0xBD: cpu.cp(cpu.l()); return 1
	case 0xBE: cpu.cp(cpu.mmu.ReadByte(cpu.hl)); return 2
	case 0xBF: cpu.cp(cpu.a); return 1

	// Misc Instructions /////////////////////////////////////////

	case 0xC0: 		// RET NZ
		return cpu.ret(!cpu.fz)
	case 0xC1:		// POP BC
		cpu.wbc(cpu.pop())
		return 12/4
	case 0xC2:		// JP NZ,a16
		return cpu.jp(!cpu.fz)
	case 0xC3:		// JP a16
		return cpu.jp(true)
	case 0xC4:		// CALL NZ,a16
		return cpu.call(!cpu.fz)
	case 0xC5:		// PUSH BC
		cpu.push(cpu.bc())
		return 16/4
	case 0xC6:		// ADD A,d8
		cpu.add(cpu.fetchByte())
		return 8/4
	case 0xC7:		// RST 00H
		return cpu.rst(0x00)
	case 0xC8:		// RET Z
		return cpu.ret(cpu.fz)
	case 0xC9:		// RET
		cpu.pc = cpu.pop()
		return 16/4
	case 0xCA:		// JP Z,a16
		return cpu.jp(cpu.fz)
	case 0xCB:		// ** PREFIX CB **
		return cpu.fdxCB()
	case 0xCC:		// CALL Z,a16
		return cpu.call(cpu.fz)
	case 0xCD:		// CALL a16
		return cpu.call(true)
	case 0xCE:		// ADC A,d8
		cpu.adc(cpu.fetchByte())
		return 8/4
	case 0xCF:		// RST 08H
		return cpu.rst(0x08)

	case 0xD0:		// RET NC
		return cpu.ret(!cpu.fc)
	case 0xD1:		// POP DE
		cpu.wde(cpu.pop())
		return 12/4
	case 0xD2:		// JP NC,a16
		return cpu.jp(!cpu.fc)
	case 0xD3: panic("cpu: invalid opcode 0xD3")
	case 0xD4: 		// CALL NC,a16
		return cpu.call(!cpu.fc)
	case 0xD5:		// PUSH DE
		cpu.push(cpu.de())
		return 16/4
	case 0xD6:		// SUB d8
		cpu.sub(cpu.fetchByte())
		return 8/4
	case 0xD7:		// RST 10H
		return cpu.rst(0x10)
	case 0xD8:		// RET C
		return cpu.ret(cpu.fc)
	case 0xD9:		// RETI
		cpu.ime = true
		cpu.pc = cpu.pop()
		return 16/4
	case 0xDA:		// JP C,a16
		return cpu.jp(cpu.fc)
	case 0xDB: panic("cpu: invalid opcode 0xDB")
	case 0xDC:		// CALL C,a16
		return cpu.call(cpu.fc)
	case 0xDD: panic("cpu: invalid opcode 0xDD")
	case 0xDE:		// SBC d8
		cpu.sbc(cpu.fetchByte())
		return 8/4
	case 0xDF:		// RST 18H
		return cpu.rst(0x18)

	case 0xE0:		// LDH (a8),A
		addr := 0xFF00 + uint16(cpu.fetchByte())
		cpu.mmu.WritePort(addr, cpu.a)
		return 12/4
	case 0xE1:		// POP HL
		cpu.hl = cpu.pop()
		return 12/4
	case 0xE2:		// LD (C),A
		addr := 0xFF00 + uint16(cpu.c)
		cpu.mmu.WritePort(addr, cpu.a)
		return 8/4
	case 0xE3: panic("cpu: invalid opcode 0xE3")
	case 0xE4: panic("cpu: invalid opcode 0xE4")
	case 0xE5:		// PUSH HL
		cpu.push(cpu.hl)
		return 16/4
	case 0xE6:		// AND d8
		cpu.and(cpu.fetchByte())
		return 8/4
	case 0xE7:		// RST 20H
		return cpu.rst(0x20)
	case 0xE8:		// ADD SP,r8
		x := cpu.fetchByte()
		cpu.sp = cpu.add16(cpu.sp, uint16(int8(x)))
		return 16/4
	case 0xE9:		// JP (HL)
		cpu.pc = cpu.hl
		return 4/4
	case 0xEA:		// LD (a16),A
		cpu.mmu.WriteByte(cpu.fetchWord(), cpu.a)
		return 16/4
	case 0xEB: panic("cpu: invalid opcode 0xEB")
	case 0xEC: panic("cpu: invalid opcode 0xEC")
	case 0xED: panic("cpu: invalid opcode 0xED")
	case 0xEE:		// XOR d8
		cpu.xor(cpu.fetchByte())
		return 8/4
	case 0xEF: 		// RST 28H
		return cpu.rst(0x28)

	case 0xF0:		// LDH A,(a8)
		addr := 0xFF00 + uint16(cpu.fetchByte())
		cpu.a = cpu.mmu.ReadPort(addr)
		return 12/4
	case 0xF1:		// POP AF
		cpu.waf(cpu.pop())
		return 12/4
	case 0xF2:		// LD A,(C)
		addr := 0xFF00 + uint16(cpu.c)
		cpu.a = cpu.mmu.ReadPort(addr)
		return 8/4
	case 0xF3:		// DI
		cpu.ime = false
		return 4/4
	case 0xF4: panic("cpu: invalid opcode 0xF4")
	case 0xF5:		// PUSH AF
		cpu.push(cpu.af())
		return 16/4
	case 0xF6:		// OR d8
		cpu.or(cpu.fetchByte())
		return 8/4
	case 0xF7:		// RST 30H
		return cpu.rst(0x30)
	case 0xF8:		// LD HL,SP+r8
		x := cpu.fetchByte()
		cpu.hl = cpu.add16(cpu.sp, uint16(int8(x)))
		return 12/4
	case 0xF9:		// LD SP,HL
		cpu.sp = cpu.hl
		return 8/4
	case 0xFA:		// LD A,(a16)
		cpu.a = cpu.mmu.ReadByte(cpu.fetchWord())
		return 16/4
	case 0xFB:		// EI
		cpu.ime = true
		return 4/4
	case 0xFC: panic("cpu: invalid opcode 0xFC")
	case 0xFD: panic("cpu: invalid opcode 0xFD")
	case 0xFE: 		// CP d8
		cpu.cp(cpu.fetchByte())
		return 8/4
	case 0xFF:		// RST 38H
		return cpu.rst(0x38)
	}
	panic("unreachable in cpu.Interpret")
}

func (cpu *CPU) fdxCB() int {
	switch cpu.fetchByte() {
	case 0x00: cpu.b = cpu.rlc(cpu.b); return 2
	case 0x01: cpu.c = cpu.rlc(cpu.c); return 2
	case 0x02: cpu.d = cpu.rlc(cpu.d); return 2
	case 0x03: cpu.e = cpu.rlc(cpu.e); return 2
	case 0x04: cpu.wh(cpu.rlc(cpu.h())); return 2
	case 0x05: cpu.wl(cpu.rlc(cpu.l())); return 2
	case 0x06: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, cpu.rlc(x))
		   return 4
	case 0x07: cpu.a = cpu.rlc(cpu.a); return 2

	case 0x08: cpu.b = cpu.rrc(cpu.b); return 2
	case 0x09: cpu.c = cpu.rrc(cpu.c); return 2
	case 0x0A: cpu.d = cpu.rrc(cpu.d); return 2
	case 0x0B: cpu.e = cpu.rrc(cpu.e); return 2
	case 0x0C: cpu.wh(cpu.rrc(cpu.h())); return 2
	case 0x0D: cpu.wl(cpu.rrc(cpu.l())); return 2
	case 0x0E: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, cpu.rrc(x))
		   return 4
	case 0x0F: cpu.a = cpu.rrc(cpu.a); return 2

	case 0x10: cpu.b = cpu.rl(cpu.b); return 2
	case 0x11: cpu.c = cpu.rl(cpu.c); return 2
	case 0x12: cpu.d = cpu.rl(cpu.d); return 2
	case 0x13: cpu.e = cpu.rl(cpu.e); return 2
	case 0x14: cpu.wh(cpu.rl(cpu.h())); return 2
	case 0x15: cpu.wl(cpu.rl(cpu.l())); return 2
	case 0x16: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, cpu.rl(x))
		   return 4
	case 0x17: cpu.a = cpu.rl(cpu.a); return 2

	case 0x18: cpu.b = cpu.rr(cpu.b); return 2
	case 0x19: cpu.c = cpu.rr(cpu.c); return 2
	case 0x1A: cpu.d = cpu.rr(cpu.d); return 2
	case 0x1B: cpu.e = cpu.rr(cpu.e); return 2
	case 0x1C: cpu.wh(cpu.rr(cpu.h())); return 2
	case 0x1D: cpu.wl(cpu.rr(cpu.l())); return 2
	case 0x1E: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, cpu.rr(x))
		   return 4
	case 0x1F: cpu.a = cpu.rr(cpu.a); return 2

	case 0x20: cpu.b = cpu.sla(cpu.b); return 2
	case 0x21: cpu.c = cpu.sla(cpu.c); return 2
	case 0x22: cpu.d = cpu.sla(cpu.d); return 2
	case 0x23: cpu.e = cpu.sla(cpu.e); return 2
	case 0x24: cpu.wh(cpu.sla(cpu.h())); return 2
	case 0x25: cpu.wl(cpu.sla(cpu.l())); return 2
	case 0x26: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, cpu.sla(x))
		   return 4
	case 0x27: cpu.a = cpu.sla(cpu.a); return 2

	case 0x28: cpu.b = cpu.sra(cpu.b); return 2
	case 0x29: cpu.c = cpu.sra(cpu.c); return 2
	case 0x2A: cpu.d = cpu.sra(cpu.d); return 2
	case 0x2B: cpu.e = cpu.sra(cpu.e); return 2
	case 0x2C: cpu.wh(cpu.sra(cpu.h())); return 2
	case 0x2D: cpu.wl(cpu.sra(cpu.l())); return 2
	case 0x2E: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, cpu.sra(x))
		   return 4
	case 0x2F: cpu.a = cpu.sra(cpu.a); return 2

	case 0x30: cpu.b = cpu.swap(cpu.b); return 2
	case 0x31: cpu.c = cpu.swap(cpu.c); return 2
	case 0x32: cpu.d = cpu.swap(cpu.d); return 2
	case 0x33: cpu.e = cpu.swap(cpu.e); return 2
	case 0x34: cpu.wh(cpu.swap(cpu.h())); return 2
	case 0x35: cpu.wl(cpu.swap(cpu.l())); return 2
	case 0x36: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, cpu.swap(x))
		   return 4
	case 0x37: cpu.a = cpu.swap(cpu.a); return 2

	case 0x38: cpu.b = cpu.srl(cpu.b); return 2
	case 0x39: cpu.c = cpu.srl(cpu.c); return 2
	case 0x3A: cpu.d = cpu.srl(cpu.d); return 2
	case 0x3B: cpu.e = cpu.srl(cpu.e); return 2
	case 0x3C: cpu.wh(cpu.srl(cpu.h())); return 2
	case 0x3D: cpu.wl(cpu.srl(cpu.l())); return 2
	case 0x3E: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, cpu.srl(x))
		   return 4
	case 0x3F: cpu.a = cpu.srl(cpu.a); return 2

	case 0x40: cpu.bit(0, cpu.b); return 2
	case 0x41: cpu.bit(0, cpu.c); return 2
	case 0x42: cpu.bit(0, cpu.d); return 2
	case 0x43: cpu.bit(0, cpu.e); return 2
	case 0x44: cpu.bit(0, cpu.h()); return 2
	case 0x45: cpu.bit(0, cpu.l()); return 2
	case 0x46: cpu.bit(0, cpu.mmu.ReadByte(cpu.hl)); return 4
	case 0x47: cpu.bit(0, cpu.a); return 2

	case 0x48: cpu.bit(1, cpu.b); return 2
	case 0x49: cpu.bit(1, cpu.c); return 2
	case 0x4A: cpu.bit(1, cpu.d); return 2
	case 0x4B: cpu.bit(1, cpu.e); return 2
	case 0x4C: cpu.bit(1, cpu.h()); return 2
	case 0x4D: cpu.bit(1, cpu.l()); return 2
	case 0x4E: cpu.bit(1, cpu.mmu.ReadByte(cpu.hl)); return 4
	case 0x4F: cpu.bit(1, cpu.a); return 2

	case 0x50: cpu.bit(2, cpu.b); return 2
	case 0x51: cpu.bit(2, cpu.c); return 2
	case 0x52: cpu.bit(2, cpu.d); return 2
	case 0x53: cpu.bit(2, cpu.e); return 2
	case 0x54: cpu.bit(2, cpu.h()); return 2
	case 0x55: cpu.bit(2, cpu.l()); return 2
	case 0x56: cpu.bit(2, cpu.mmu.ReadByte(cpu.hl)); return 4
	case 0x57: cpu.bit(2, cpu.a); return 2

	case 0x58: cpu.bit(3, cpu.b); return 2
	case 0x59: cpu.bit(3, cpu.c); return 2
	case 0x5A: cpu.bit(3, cpu.d); return 2
	case 0x5B: cpu.bit(3, cpu.e); return 2
	case 0x5C: cpu.bit(3, cpu.h()); return 2
	case 0x5D: cpu.bit(3, cpu.l()); return 2
	case 0x5E: cpu.bit(3, cpu.mmu.ReadByte(cpu.hl)); return 4
	case 0x5F: cpu.bit(3, cpu.a); return 2

	case 0x60: cpu.bit(4, cpu.b); return 2
	case 0x61: cpu.bit(4, cpu.c); return 2
	case 0x62: cpu.bit(4, cpu.d); return 2
	case 0x63: cpu.bit(4, cpu.e); return 2
	case 0x64: cpu.bit(4, cpu.h()); return 2
	case 0x65: cpu.bit(4, cpu.l()); return 2
	case 0x66: cpu.bit(4, cpu.mmu.ReadByte(cpu.hl)); return 4
	case 0x67: cpu.bit(4, cpu.a); return 2

	case 0x68: cpu.bit(5, cpu.b); return 2
	case 0x69: cpu.bit(5, cpu.c); return 2
	case 0x6A: cpu.bit(5, cpu.d); return 2
	case 0x6B: cpu.bit(5, cpu.e); return 2
	case 0x6C: cpu.bit(5, cpu.h()); return 2
	case 0x6D: cpu.bit(5, cpu.l()); return 2
	case 0x6E: cpu.bit(5, cpu.mmu.ReadByte(cpu.hl)); return 4
	case 0x6F: cpu.bit(5, cpu.a); return 2

	case 0x70: cpu.bit(6, cpu.b); return 2
	case 0x71: cpu.bit(6, cpu.c); return 2
	case 0x72: cpu.bit(6, cpu.d); return 2
	case 0x73: cpu.bit(6, cpu.e); return 2
	case 0x74: cpu.bit(6, cpu.h()); return 2
	case 0x75: cpu.bit(6, cpu.l()); return 2
	case 0x76: cpu.bit(6, cpu.mmu.ReadByte(cpu.hl)); return 4
	case 0x77: cpu.bit(6, cpu.a); return 2

	case 0x78: cpu.bit(7, cpu.b); return 2
	case 0x79: cpu.bit(7, cpu.c); return 2
	case 0x7A: cpu.bit(7, cpu.d); return 2
	case 0x7B: cpu.bit(7, cpu.e); return 2
	case 0x7C: cpu.bit(7, cpu.h()); return 2
	case 0x7D: cpu.bit(7, cpu.l()); return 2
	case 0x7E: cpu.bit(7, cpu.mmu.ReadByte(cpu.hl)); return 4
	case 0x7F: cpu.bit(7, cpu.a); return 2

	case 0x80: cpu.b = res(0, cpu.b); return 2
	case 0x81: cpu.c = res(0, cpu.c); return 2
	case 0x82: cpu.d = res(0, cpu.d); return 2
	case 0x83: cpu.e = res(0, cpu.e); return 2
	case 0x84: cpu.wh(res(0, cpu.h())); return 2
	case 0x85: cpu.wl(res(0, cpu.l())); return 2
	case 0x86: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, res(0, x))
		   return 4
	case 0x87: cpu.a = res(0, cpu.a); return 2

	case 0x88: cpu.b = res(1, cpu.b); return 2
	case 0x89: cpu.c = res(1, cpu.c); return 2
	case 0x8A: cpu.d = res(1, cpu.d); return 2
	case 0x8B: cpu.e = res(1, cpu.e); return 2
	case 0x8C: cpu.wh(res(1, cpu.h())); return 2
	case 0x8D: cpu.wl(res(1, cpu.l())); return 2
	case 0x8E: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, res(1, x))
		   return 4
	case 0x8F: cpu.a = res(1, cpu.a); return 2

	case 0x90: cpu.b = res(2, cpu.b); return 2
	case 0x91: cpu.c = res(2, cpu.c); return 2
	case 0x92: cpu.d = res(2, cpu.d); return 2
	case 0x93: cpu.e = res(2, cpu.e); return 2
	case 0x94: cpu.wh(res(2, cpu.h())); return 2
	case 0x95: cpu.wl(res(2, cpu.l())); return 2
	case 0x96: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, res(2, x))
		   return 4
	case 0x97: cpu.a = res(2, cpu.a); return 2

	case 0x98: cpu.b = res(3, cpu.b); return 2
	case 0x99: cpu.c = res(3, cpu.c); return 2
	case 0x9A: cpu.d = res(3, cpu.d); return 2
	case 0x9B: cpu.e = res(3, cpu.e); return 2
	case 0x9C: cpu.wh(res(3, cpu.h())); return 2
	case 0x9D: cpu.wl(res(3, cpu.l())); return 2
	case 0x9E: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, res(3, x))
		   return 4
	case 0x9F: cpu.a = res(3, cpu.a); return 2

	case 0xA0: cpu.b = res(4, cpu.b); return 2
	case 0xA1: cpu.c = res(4, cpu.c); return 2
	case 0xA2: cpu.d = res(4, cpu.d); return 2
	case 0xA3: cpu.e = res(4, cpu.e); return 2
	case 0xA4: cpu.wh(res(4, cpu.h())); return 2
	case 0xA5: cpu.wl(res(4, cpu.l())); return 2
	case 0xA6: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, res(4, x))
		   return 4
	case 0xA7: cpu.a = res(4, cpu.a); return 2

	case 0xA8: cpu.b = res(5, cpu.b); return 2
	case 0xA9: cpu.c = res(5, cpu.c); return 2
	case 0xAA: cpu.d = res(5, cpu.d); return 2
	case 0xAB: cpu.e = res(5, cpu.e); return 2
	case 0xAC: cpu.wh(res(5, cpu.h())); return 2
	case 0xAD: cpu.wl(res(5, cpu.l())); return 2
	case 0xAE: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, res(5, x))
		   return 4
	case 0xAF: cpu.a = res(5, cpu.a); return 2

	case 0xB0: cpu.b = res(6, cpu.b); return 2
	case 0xB1: cpu.c = res(6, cpu.c); return 2
	case 0xB2: cpu.d = res(6, cpu.d); return 2
	case 0xB3: cpu.e = res(6, cpu.e); return 2
	case 0xB4: cpu.wh(res(6, cpu.h())); return 2
	case 0xB5: cpu.wl(res(6, cpu.l())); return 2
	case 0xB6: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, res(6, x))
		   return 4
	case 0xB7: cpu.a = res(6, cpu.a); return 2

	case 0xB8: cpu.b = res(7, cpu.b); return 2
	case 0xB9: cpu.c = res(7, cpu.c); return 2
	case 0xBA: cpu.d = res(7, cpu.d); return 2
	case 0xBB: cpu.e = res(7, cpu.e); return 2
	case 0xBC: cpu.wh(res(7, cpu.h())); return 2
	case 0xBD: cpu.wl(res(7, cpu.l())); return 2
	case 0xBE: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, res(7, x))
		   return 4
	case 0xBF: cpu.a = res(7, cpu.a); return 2

	case 0xC0: cpu.b = set(0, cpu.b); return 2
	case 0xC1: cpu.c = set(0, cpu.c); return 2
	case 0xC2: cpu.d = set(0, cpu.d); return 2
	case 0xC3: cpu.e = set(0, cpu.e); return 2
	case 0xC4: cpu.wh(set(0, cpu.h())); return 2
	case 0xC5: cpu.wl(set(0, cpu.l())); return 2
	case 0xC6: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, set(0, x))
		   return 4
	case 0xC7: cpu.a = set(0, cpu.a); return 2

	case 0xC8: cpu.b = set(1, cpu.b); return 2
	case 0xC9: cpu.c = set(1, cpu.c); return 2
	case 0xCA: cpu.d = set(1, cpu.d); return 2
	case 0xCB: cpu.e = set(1, cpu.e); return 2
	case 0xCC: cpu.wh(set(1, cpu.h())); return 2
	case 0xCD: cpu.wl(set(1, cpu.l())); return 2
	case 0xCE: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, set(1, x))
		   return 4
	case 0xCF: cpu.a = set(1, cpu.a); return 2

	case 0xD0: cpu.b = set(2, cpu.b); return 2
	case 0xD1: cpu.c = set(2, cpu.c); return 2
	case 0xD2: cpu.d = set(2, cpu.d); return 2
	case 0xD3: cpu.e = set(2, cpu.e); return 2
	case 0xD4: cpu.wh(set(2, cpu.h())); return 2
	case 0xD5: cpu.wl(set(2, cpu.l())); return 2
	case 0xD6: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, set(2, x))
		   return 4
	case 0xD7: cpu.a = set(2, cpu.a); return 2

	case 0xD8: cpu.b = set(3, cpu.b); return 2
	case 0xD9: cpu.c = set(3, cpu.c); return 2
	case 0xDA: cpu.d = set(3, cpu.d); return 2
	case 0xDB: cpu.e = set(3, cpu.e); return 2
	case 0xDC: cpu.wh(set(3, cpu.h())); return 2
	case 0xDD: cpu.wl(set(3, cpu.l())); return 2
	case 0xDE: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, set(3, x))
		   return 4
	case 0xDF: cpu.a = set(3, cpu.a); return 2

	case 0xE0: cpu.b = set(4, cpu.b); return 2
	case 0xE1: cpu.c = set(4, cpu.c); return 2
	case 0xE2: cpu.d = set(4, cpu.d); return 2
	case 0xE3: cpu.e = set(4, cpu.e); return 2
	case 0xE4: cpu.wh(set(4, cpu.h())); return 2
	case 0xE5: cpu.wl(set(4, cpu.l())); return 2
	case 0xE6: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, set(4, x))
		   return 4
	case 0xE7: cpu.a = set(4, cpu.a); return 2

	case 0xE8: cpu.b = set(5, cpu.b); return 2
	case 0xE9: cpu.c = set(5, cpu.c); return 2
	case 0xEA: cpu.d = set(5, cpu.d); return 2
	case 0xEB: cpu.e = set(5, cpu.e); return 2
	case 0xEC: cpu.wh(set(5, cpu.h())); return 2
	case 0xED: cpu.wl(set(5, cpu.l())); return 2
	case 0xEE: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, set(5, x))
		   return 4
	case 0xEF: cpu.a = set(5, cpu.a); return 2

	case 0xF0: cpu.b = set(6, cpu.b); return 2
	case 0xF1: cpu.c = set(6, cpu.c); return 2
	case 0xF2: cpu.d = set(6, cpu.d); return 2
	case 0xF3: cpu.e = set(6, cpu.e); return 2
	case 0xF4: cpu.wh(set(6, cpu.h())); return 2
	case 0xF5: cpu.wl(set(6, cpu.l())); return 2
	case 0xF6: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, set(6, x))
		   return 4
	case 0xF7: cpu.a = set(6, cpu.a); return 2

	case 0xF8: cpu.b = set(7, cpu.b); return 2
	case 0xF9: cpu.c = set(7, cpu.c); return 2
	case 0xFA: cpu.d = set(7, cpu.d); return 2
	case 0xFB: cpu.e = set(7, cpu.e); return 2
	case 0xFC: cpu.wh(set(7, cpu.h())); return 2
	case 0xFD: cpu.wl(set(7, cpu.l())); return 2
	case 0xFE: x := cpu.mmu.ReadByte(cpu.hl)
		   cpu.mmu.WriteByte(cpu.hl, set(7, x))
		   return 4
	case 0xFF: cpu.a = set(7, cpu.a); return 2
	}
	panic("unreachable in cpu.Interpret (CB)")
}

func (cpu *CPU) jr(pred bool) int {
	x := cpu.fetchByte()
	if pred {
		cpu.pc += uint16(int8(x))
		return 12/4
	}
	return 8/4
}

func (cpu *CPU) ret(pred bool) int {
	if pred {
		cpu.pc = cpu.pop()
		return 20/4
	}
	return 8/4
}

func (cpu *CPU) jp(pred bool) int {
	x := cpu.fetchWord()
	if pred {
		cpu.pc = x
		return 16/4
	}
	return 12/4
}

func (cpu *CPU) call(pred bool) int {
	x := cpu.fetchWord()
	if pred {
		cpu.push(cpu.pc)
		cpu.pc = x
		return 24/4
	}
	return 12/4
}

func (cpu *CPU) rst(addr uint16) int {
	cpu.push(cpu.pc)
	cpu.pc = addr
	return 32/4
}

func (cpu *CPU) push(x uint16) {
	cpu.sp -= 2
	cpu.mmu.WriteWord(cpu.sp, x)
	//fmt.Printf("-> SP=%04Xh *=%04Xh\n", cpu.sp, x)
}

func (cpu *CPU) pop() uint16 {
	x := cpu.mmu.ReadWord(cpu.sp)
	//fmt.Printf("<- SP=%04Xh *=%04Xh\n", cpu.sp, x)
	cpu.sp += 2
	return x
}

func (cpu *CPU) inc(x byte) byte {
	y := x + 1
	cpu.fz = y == 0
	cpu.fn = false
	cpu.fh = (y & 0x0F) == 0
	return y
}

func (cpu *CPU) dec(x byte) byte {
	y := x - 1
	cpu.fz = y == 0
	cpu.fn = true
	cpu.fh = (y & 0x0F) == 0x0F
	return y
}

func (cpu *CPU) add16(x, y uint16) uint16 {
	x1 := x + y
	cpu.fn = false
	cpu.fh = x1 & 0x0FFF < x & 0x0FFF
	cpu.fc = x1 < x
	return x1
}

func (cpu *CPU) add(x byte) {
	y := cpu.a + x
	cpu.fz = y == 0
	cpu.fn = false
	cpu.fh = y & 0x0F < cpu.a & 0x0F
	cpu.fc = y < cpu.a
	cpu.a = y
}

func (cpu *CPU) adc(x byte) {
	fc := byte(0)
	if cpu.fc { fc = 1 }
	y := cpu.a + x + fc
	cpu.fz = y == 0
	cpu.fn = false
	cpu.fh = y & 0x0F < cpu.a & 0x0F
	cpu.fc = y < cpu.a
	cpu.a = y
}

func (cpu *CPU) sub(x byte) {
	y := cpu.a - x
	cpu.fz = y == 0
	cpu.fn = true
	cpu.fh = y & 0x0F > cpu.a & 0x0F
	cpu.fc = y > cpu.a
	cpu.a = y
}

func (cpu *CPU) sbc(x byte) {
	fc := byte(0)
	if cpu.fc { fc = 1 }
	y := cpu.a - x - fc
	cpu.fz = y == 0
	cpu.fn = true
	cpu.fh = y & 0x0F > cpu.a & 0x0F
	cpu.fc = y > cpu.a
	cpu.a = y
}

func (cpu *CPU) and(x byte) {
	cpu.a &= x
	cpu.fz = cpu.a == 0
	cpu.fn = false
	cpu.fh = true
	cpu.fc = false
}

func (cpu *CPU) xor(x byte) {
	cpu.a ^= x
	cpu.fz = cpu.a == 0
	cpu.fn = false
	cpu.fh = false
	cpu.fc = false
}

func (cpu *CPU) or(x byte) {
	cpu.a |= x
	cpu.fz = cpu.a == 0
	cpu.fn = false
	cpu.fh = false
	cpu.fc = false
}

func (cpu *CPU) cp(x byte) {
	y := cpu.a - x
	cpu.fz = y == 0
	cpu.fn = true
	cpu.fh = y & 0x0F > cpu.a & 0x0F
	cpu.fc = y > cpu.a
}

func (cpu *CPU) rlc(x byte) byte {
	fc := x >> 7
	y := x << 1 | fc
	cpu.fz = y == 0
	cpu.fn = false
	cpu.fh = false
	cpu.fc = fc == 1
	return y
}

func (cpu *CPU) rrc(x byte) byte {
	fc := x << 7
	y := x >> 1 | fc
	cpu.fz = y == 0
	cpu.fn = false
	cpu.fh = false
	cpu.fc = fc == 0x80
	return y
}

func (cpu *CPU) rl(x byte) byte {
	fc := byte(0)
	if cpu.fc { fc = 1 }
	y := x << 1 | fc
	cpu.fz = y == 0
	cpu.fn = false
	cpu.fh = false
	cpu.fc = x & 1 == 1
	return y
}

func (cpu *CPU) rr(x byte) byte {
	fc := byte(0)
	if cpu.fc { fc = 0x80 }
	y := x >> 1 | fc
	cpu.fz = y == 0
	cpu.fn = false
	cpu.fh = false
	cpu.fc = x & 0x80 == 0x80
	return y
}

func (cpu *CPU) sla(x byte) byte {
	y := x << 1
	cpu.fz = y == 0
	cpu.fn = false
	cpu.fh = false
	cpu.fc = x & 0x80 == 0x80
	return y
}

func (cpu *CPU) sra(x byte) byte {
	y := byte(int8(x) >> 1)
	cpu.fz = y == 0
	cpu.fn = false
	cpu.fh = false
	cpu.fc = x & 1 == 1
	return y
}

func (cpu *CPU) srl(x byte) byte {
	y := x >> 1
	cpu.fz = y == 0
	cpu.fn = false
	cpu.fh = false
	cpu.fc = x & 1 == 1
	return y
}

func (cpu *CPU) swap(x byte) byte {
	y := x >> 4 | x << 4
	cpu.fz = y == 0
	cpu.fn = false
	cpu.fh = false
	cpu.fc = false
	return y
}

func (cpu *CPU) bit(n, x byte) {
	cpu.fz = x & (1 << n) != 0
	cpu.fn = false
	cpu.fh = true
}

func res(n, x byte) byte {
	return x &^ (1 << n)
}

func set(n, x byte) byte {
	return x | 1 << n
}

// DAA and DAS implementations based on pseudocode from 80386
// instruction set references.

func (cpu *CPU) daa() {
	if cpu.a & 0x0F > 9 || cpu.fh {
		cpu.a += 6
		cpu.fh = true
	} else {
		cpu.fh = false
	}
	if cpu.a > 0x9F || cpu.fc {
		cpu.a += 0x60
		cpu.fc = true
	} else {
		cpu.fc = false
	}
	cpu.fz = cpu.a == 0
}

func (cpu *CPU) das() {
	if cpu.a & 0x0F > 9 || cpu.fh {
		cpu.a -= 6
		cpu.fh = true
	} else {
		cpu.fh = false
	}
	if cpu.a > 0x9F || cpu.fc {
		cpu.a -= 0x60
		cpu.fc = true
	} else {
		cpu.fc = false
	}
	cpu.fz = cpu.a == 0
}

func (cpu *CPU) af() uint16 {
	a := uint16(cpu.a)
	f := uint16(0)
	if cpu.fz { f |= 0x80 }
	if cpu.fn { f |= 0x40 }
	if cpu.fh { f |= 0x20 }
	if cpu.fc { f |= 0x10 }
	return (a << 8) | f
}

func (cpu *CPU) waf(x uint16) {
	cpu.a = byte(x >> 8)
	cpu.fz = (x & 0x80) == 0x80
	cpu.fn = (x & 0x40) == 0x40
	cpu.fh = (x & 0x20) == 0x20
	cpu.fc = (x & 0x10) == 0x10
}

func (cpu *CPU) bc() uint16 {
	b := uint16(cpu.b)
	c := uint16(cpu.c)
	return (b << 8) | c
}

func (cpu *CPU) wbc(x uint16) {
	cpu.b = byte(x >> 8)
	cpu.c = byte(x)
}

func (cpu *CPU) de() uint16 {
	d := uint16(cpu.d)
	e := uint16(cpu.e)
	return (d << 8) | e
}

func (cpu *CPU) wde(x uint16) {
	cpu.d = byte(x >> 8)
	cpu.e = byte(x)
}

func (cpu *CPU) h() byte {
	return byte(cpu.hl >> 8)
}

func (cpu *CPU) wh(x byte) {
	cpu.hl = (uint16(x) << 8) | (cpu.hl & 0xFF)
}

func (cpu *CPU) l() byte {
	return byte(cpu.hl)
}

func (cpu *CPU) wl(x byte) {
	cpu.hl = uint16(x) | (cpu.hl & 0xFF00)
}

func (cpu *CPU) fetchByte() byte {
	pc := cpu.pc
	cpu.pc++
	return cpu.mmu.ReadByte(pc)
}

func (cpu *CPU) fetchWord() uint16 {
	pc := cpu.pc
	cpu.pc += 2
	return cpu.mmu.ReadWord(pc)
}
