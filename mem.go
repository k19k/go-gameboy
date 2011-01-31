package main

import (
	"fmt"
	"io"
)

const (
	PortJOYP = 0xFF00
	PortSB   = 0xFF01
	PortSC   = 0xFF02
	PortDIV  = 0xFF04
	PortTIMA = 0xFF05
	PortTMA  = 0xFF06
	PortTAC  = 0xFF07
	PortIF   = 0xFF0F
	PortNR10 = 0xFF10
	PortNR11 = 0xFF11
	PortNR12 = 0xFF12
	PortNR13 = 0xFF13
	PortNR14 = 0xFF14
	PortNR21 = 0xFF16
	PortNR22 = 0xFF17
	PortNR23 = 0xFF18
	PortNR24 = 0xFF19
	PortNR30 = 0xFF1A
	PortNR31 = 0xFF1B
	PortNR32 = 0xFF1C
	PortNR33 = 0xFF1D
	PortNR34 = 0xFF1E
	PortNR41 = 0xFF20
	PortNR42 = 0xFF21
	PortNR43 = 0xFF22
	PortNR44 = 0xFF23
	PortNR50 = 0xFF24
	PortNR51 = 0xFF25
	PortNR52 = 0xFF26
	PortLCDC = 0xFF40
	PortSTAT = 0xFF41
	PortSCY  = 0xFF42
	PortSCX  = 0xFF43
	PortLY   = 0xFF44
	PortLYC  = 0xFF45
	PortDMA  = 0xFF46
	PortBGP  = 0xFF47
	PortOBP0 = 0xFF48
	PortOBP1 = 0xFF49
	PortWY   = 0xFF4A
	PortWX   = 0xFF4B
	PortIE   = 0xFFFF
)

const (
	VBlankAddr	= 0x0040
	LCDStatusAddr	= 0x0048
	TimerAddr	= 0x0050
	SerialAddr	= 0x0058
	JoypadAddr	= 0x0060
)

const (
	divOverflow = 64
)

type MBC struct {
	rom []byte
	vram [0x2000]byte
	eram [0x8000]byte
	wram [0x2000]byte
	oam [0xA0]byte
	hram [0x100]byte

	romBank int
	eramBank uint16
	ramMode bool

	// LCDC flags
	LCDEnable bool
	WindowMap bool
	WindowEnable bool
	TileData bool
	BGMap bool
	SpriteSize bool
	SpriteEnable bool
	BGEnable bool

	// STAT flags
	LYCInterrupt, OAMInterrupt, VBlankInterrupt, HBlankInterrupt bool
	LCDMode byte

	divTicks int
	timaTicks int
	timaOverflow int

	BGP [4]byte
	OBP [2][4]byte
}

func (m *MBC) String() string {
	return fmt.Sprintf("<MBC %p>", m)
}

func (m *MBC) Dump(w io.Writer) {
	var addr int

	read := func () (x byte, e interface{}) {
		defer func() {
			e = recover()
			addr++
		}()
		return m.ReadByte(uint16(addr)), e
	}

	fmt.Fprintf(w, "MEMORY DUMP ---- ROM BANK: %d -- ERAM BANK: %d\n",
		m.romBank, m.eramBank)

	for addr <= 0xFFFF {
		fmt.Fprintf(w, "%04x  ", addr)
		for x := 0; x < 8; x++ {
			b, e := read()
			if e == nil {
				fmt.Fprintf(w, "%02x ", b)
			} else {
				fmt.Fprint(w, "?? ")
			}
		}
		fmt.Fprint(w, " ")
		for x := 0; x < 8; x++ {
			b, e := read()
			if e == nil {
				fmt.Fprintf(w, "%02x ", b)
			} else {
				fmt.Fprint(w, "?? ")
			}
		}
		addr -= 16
		fmt.Fprint(w, " |")
		for x := 0; x < 16; x++ {
			b, e := read()
			if e != nil {
				fmt.Fprint(w, "?")
			} else if b < 0x20 || b >= 0x7F {
				fmt.Fprint(w, ".")
			} else {
				fmt.Fprintf(w, "%c", b)
			}
		}
		fmt.Fprint(w, "\n")
	}
	fmt.Fprint(w, "\n")
}

func (m *MBC) ReadByte(addr uint16) byte {
	switch {
	case addr < 0x8000:
		return m.ReadROM(addr)
	case addr < 0xA000:
		return m.ReadVideoRAM(addr)
	case addr < 0xC000:
		return m.ReadExternalRAM(addr)
	case addr < 0xE000:
		return m.ReadWorkRAM(addr)
	case addr < 0xFE00:
		return m.ReadWorkRAM(addr - 0x2000)
	case addr < 0xFEA0:
		return m.ReadOAM(addr)
	case addr >= 0xFF00:
		return m.ReadPort(addr)
	}
	return 0
}

func (m *MBC) WriteByte(addr uint16, x byte) {
	switch {
	case addr < 0x8000:
		m.WriteROM(addr, x)
	case addr < 0xA000:
		m.WriteVideoRAM(addr, x)
	case addr < 0xC000:
		m.WriteExternalRAM(addr, x)
	case addr < 0xE000:
		m.WriteWorkRAM(addr, x)
	case addr < 0xFE00:
		m.WriteWorkRAM(addr - 0x2000, x)
	case addr < 0xFEA0:
		m.WriteOAM(addr, x)
	case addr >= 0xFF00:
		m.WritePort(addr, x)
	}
}

func (m *MBC) ReadWord(addr uint16) uint16 {
	lo := uint16(m.ReadByte(addr))
	hi := uint16(m.ReadByte(addr+1))
	return (hi << 8) | lo
}

func (m *MBC) WriteWord(addr uint16, x uint16) {
	m.WriteByte(addr, uint8(x & 0xFF))
	m.WriteByte(addr+1, uint8(x >> 8))
}

func NewMBC(rom []byte) *MBC {
	mbc := &MBC{rom: rom}
	mbc.hram[0x00] = 0x3F
	mbc.WritePort(PortNR10, 0x80)
	mbc.WritePort(PortNR11, 0xBF)
	mbc.WritePort(PortNR12, 0xF3)
	mbc.WritePort(PortNR14, 0xB4)
	mbc.WritePort(PortNR21, 0x3F)
	mbc.WritePort(PortNR24, 0xBF)
	mbc.WritePort(PortNR30, 0x7F)
	mbc.WritePort(PortNR31, 0xFF)
	mbc.WritePort(PortNR32, 0x9F)
	mbc.WritePort(PortNR33, 0xBF)
	mbc.WritePort(PortNR41, 0xFF)
	mbc.WritePort(PortNR44, 0xBF)
	mbc.WritePort(PortNR50, 0x77)
	mbc.WritePort(PortNR51, 0xF3)
	mbc.WritePort(PortNR52, 0xF1)
	mbc.WritePort(PortLCDC, 0x91)
	mbc.WritePort(PortBGP,  0xFC)
	mbc.WritePort(PortOBP0, 0xFF)
	mbc.WritePort(PortOBP1, 0xFF)
	return mbc
}

func (mbc *MBC) ReadROM(addr uint16) byte {
	if addr < 0x4000 {
		return mbc.rom[addr]
	}
	return mbc.rom[int(addr) - 0x4000 + mbc.romBank * 0x4000]
}

func (mbc *MBC) WriteROM(addr uint16, x byte) {
	switch {
	case addr >= 0x6000:
		mbc.ramMode = x & 1 == 1
		if mbc.ramMode {
			mbc.romBank &= 0x1F
		} else {
			mbc.eramBank = 0
		}
	case addr >= 0x4000:
		if mbc.ramMode {
			mbc.eramBank = uint16(x) & 3
		} else {
			mbc.romBank &= 0x1F
			mbc.romBank |= (int(x) & 3) << 5
			//fmt.Printf("rom.56 = %x (%02x)\n", x, mbc.romBank)
		}
	case addr >= 0x2000:
		x &= 0x1F
		if x == 0 { x = 1 }
		mbc.romBank &= 0x60
		mbc.romBank |= int(x)
		//fmt.Printf("rom.01234 = %x (%02x)\n", x & 0x1F, mbc.romBank)
	}
	//fmt.Printf("ROM BANK = %d   RAM BANK = %d\n", mbc.romBank, mbc.eramBank)
}

func (mbc *MBC) ReadVideoRAM(addr uint16) byte {
	return mbc.vram[addr - 0x8000]
}

func (mbc *MBC) WriteVideoRAM(addr uint16, x byte) {
	mbc.vram[addr - 0x8000] = x
}

func (mbc *MBC) ReadExternalRAM(addr uint16) byte {
	return mbc.eram[addr - 0xA000 + mbc.eramBank * 0x2000]
}

func (mbc *MBC) WriteExternalRAM(addr uint16, x byte) {
	mbc.eram[addr - 0xA000 + mbc.eramBank * 0x2000] = x
}

func (mbc *MBC) ReadWorkRAM(addr uint16) byte {
	return mbc.wram[addr - 0xC000]
}

func (mbc *MBC) WriteWorkRAM(addr uint16, x byte) {
	mbc.wram[addr - 0xC000] = x
}

func (mbc *MBC) ReadOAM(addr uint16) byte {
	return mbc.oam[addr - 0xFE00]
}

func (mbc *MBC) WriteOAM(addr uint16, x byte) {
	mbc.oam[addr - 0xFE00] = x
}

func (mbc *MBC) ReadPort(addr uint16) byte {
	return mbc.hram[addr - 0xFF00]
}

func (mbc *MBC) WritePort(addr uint16, x byte) {
	switch addr {
	case PortJOYP:
		x = x & 0x30 | 0xF // FIXME
	case PortDIV:
		x = 0
	case PortTAC:
		switch x & 3 {
		case 0: mbc.timaOverflow = 256
		case 1: mbc.timaOverflow = 4
		case 2: mbc.timaOverflow = 16
		case 3: mbc.timaOverflow = 64
		}
		mbc.timaTicks = 0
	case PortLCDC:
		mbc.LCDEnable = x & 0x80 != 0
		mbc.WindowMap = x & 0x40 != 0
		mbc.WindowEnable = x & 0x20 != 0
		mbc.TileData = x & 0x10 != 0
		mbc.BGMap = x & 0x08 != 0
		mbc.SpriteSize = x & 0x04 != 0
		mbc.SpriteEnable = x & 0x02 != 0
		mbc.BGEnable = x & 0x01 != 0
	case PortSTAT:
		mbc.LYCInterrupt = x & 0x40 != 0
		mbc.OAMInterrupt = x & 0x20 != 0
		mbc.VBlankInterrupt = x & 0x10 != 0
		mbc.HBlankInterrupt = x & 0x08 != 0
		mbc.LCDMode = x & 0x03
	case PortBGP:
		mbc.BGP[0] = x & 3
		mbc.BGP[1] = (x >> 2) & 3
		mbc.BGP[2] = (x >> 4) & 3
		mbc.BGP[3] = (x >> 6)
	case PortOBP0:
		mbc.OBP[0][0] = x & 3
		mbc.OBP[0][1] = (x >> 2) & 3
		mbc.OBP[0][2] = (x >> 4) & 3
		mbc.OBP[0][3] = x >> 6
	case PortOBP1:
		mbc.OBP[1][0] = x & 3
		mbc.OBP[1][1] = (x >> 2) & 3
		mbc.OBP[1][2] = (x >> 4) & 3
		mbc.OBP[1][3] = x >> 6
	case PortDMA:
		src := uint16(x) << 8
		mbc.dma(src)
	}
	mbc.hram[addr - 0xFF00] = x
}

func (mbc *MBC) UpdateTimers(t int) {
	mbc.divTicks += t
	if mbc.divTicks > divOverflow {
		d := mbc.divTicks / divOverflow
		mbc.hram[0x04] += byte(d)
		mbc.divTicks -= d * divOverflow
	}
	if mbc.hram[0x07] & 4 == 0 { return }
	mbc.timaTicks += t
	if mbc.timaTicks > mbc.timaOverflow {
		d := mbc.timaTicks / mbc.timaOverflow
		x := int(mbc.hram[0x05]) + d
		if x > 0xFF {
			mbc.hram[0x05] = mbc.hram[0x06] + byte(x)
			mbc.hram[0x0F] |= 0x04
		} else {
			mbc.hram[0x05] = byte(x)
		}
		mbc.timaTicks -= d * mbc.timaOverflow
	}
}

func (mbc *MBC) dma(src uint16) {
	for i := 0; i < 0xA0; i++ {
		mbc.oam[i] = mbc.ReadByte(src)
		src++
	}
}
