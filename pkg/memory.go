// Copyright 2011 Kevin Bulusek. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gameboy

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"⚛sdl"
)

const (
	portJOYP = 0xFF00
	portSB   = 0xFF01
	portSC   = 0xFF02
	portDIV  = 0xFF04
	portTIMA = 0xFF05
	portTMA  = 0xFF06
	portTAC  = 0xFF07
	portIF   = 0xFF0F
	portNR10 = 0xFF10
	portNR11 = 0xFF11
	portNR12 = 0xFF12
	portNR13 = 0xFF13
	portNR14 = 0xFF14
	portNR21 = 0xFF16
	portNR22 = 0xFF17
	portNR23 = 0xFF18
	portNR24 = 0xFF19
	portNR30 = 0xFF1A
	portNR31 = 0xFF1B
	portNR32 = 0xFF1C
	portNR33 = 0xFF1D
	portNR34 = 0xFF1E
	portNR41 = 0xFF20
	portNR42 = 0xFF21
	portNR43 = 0xFF22
	portNR44 = 0xFF23
	portNR50 = 0xFF24
	portNR51 = 0xFF25
	portNR52 = 0xFF26
	portWAVE = 0xFF30
	portLCDC = 0xFF40
	portSTAT = 0xFF41
	portSCY  = 0xFF42
	portSCX  = 0xFF43
	portLY   = 0xFF44
	portLYC  = 0xFF45
	portDMA  = 0xFF46
	portBGP  = 0xFF47
	portOBP0 = 0xFF48
	portOBP1 = 0xFF49
	portWY   = 0xFF4A
	portWX   = 0xFF4B
	portIE   = 0xFFFF
)

const (
	vblankAddr    = 0x0040
	lcdStatusAddr = 0x0048
	timerAddr     = 0x0050
	serialAddr    = 0x0058
	joypadAddr    = 0x0060
)

const (
	divOverflow = 64
)

type memory struct {
	rom  romImage
	vram [0x2000]byte
	eram [0x8000]byte
	wram [0x2000]byte
	oam  [0xA0]byte
	hram [0x100]byte

	romBank  int
	romBanks int
	eramBank uint16
	ramMode  bool

	mbcType int

	config *Config
	quit   bool

	sys   *cpu
	lcd   *display
	audio *mixer

	divTicks     int
	timaTicks    int
	timaOverflow int

	dpadBits byte
	btnBits  byte
}

func newMemory(rom romImage, cfg *Config) (m *memory, err interface{}) {
	m = &memory{rom: rom, romBank: 1, config: cfg,
		dpadBits: 0xF, btnBits: 0xF}
	m.mbcType, err = rom.mbcType()
	if err != nil {
		return
	}
	m.romBanks, err = rom.banks()
	return
}

func (m *memory) connect(sys *cpu, lcd *display, audio *mixer) {
	m.sys = sys
	m.lcd = lcd
	m.audio = audio
	m.initPorts()
}

func (m *memory) initPorts() {
	m.writePort(portJOYP, 0x30)
	m.writePort(portNR10, 0x80)
	m.writePort(portNR11, 0xBF)
	m.writePort(portNR12, 0xF3)
	m.writePort(portNR14, 0x24) //0xB4)
	m.writePort(portNR21, 0x3F)
	m.writePort(portNR24, 0x2F) //0xBF)
	m.writePort(portNR30, 0x7F)
	m.writePort(portNR31, 0xFF)
	m.writePort(portNR32, 0x9F)
	m.writePort(portNR33, 0xBF)
	m.writePort(portNR41, 0xFF)
	m.writePort(portNR44, 0x2F) //0xBF)
	m.writePort(portNR50, 0x77)
	m.writePort(portNR51, 0xF3)
	m.writePort(portNR52, 0xF1)
	m.writePort(portLCDC, 0x91)
	m.writePort(portBGP, 0xFC)
	m.writePort(portOBP0, 0xFF)
	m.writePort(portOBP1, 0xFF)
}

func (m *memory) readByte(addr uint16) byte {
	switch {
	case addr < 0x8000:
		return m.readROM(addr)
	case addr < 0xA000:
		return m.readVideoRAM(addr)
	case addr < 0xC000:
		return m.readExternalRAM(addr)
	case addr < 0xE000:
		return m.readWorkRAM(addr)
	case addr < 0xFE00:
		return m.readWorkRAM(addr - 0x2000)
	case addr < 0xFEA0:
		return m.readOAM(addr)
	case addr >= 0xFF00:
		return m.readPort(addr)
	}
	return 0
}

func (m *memory) writeByte(addr uint16, x byte) {
	switch {
	case addr < 0x8000:
		m.writeROM(addr, x)
	case addr < 0xA000:
		m.writeVideoRAM(addr, x)
	case addr < 0xC000:
		m.writeExternalRAM(addr, x)
	case addr < 0xE000:
		m.writeWorkRAM(addr, x)
	case addr < 0xFE00:
		m.writeWorkRAM(addr-0x2000, x)
	case addr < 0xFEA0:
		m.writeOAM(addr, x)
	case addr >= 0xFF00:
		m.writePort(addr, x)
	}
}

func (m *memory) readWord(addr uint16) uint16 {
	lo := uint16(m.readByte(addr))
	hi := uint16(m.readByte(addr + 1))
	return (hi << 8) | lo
}

func (m *memory) writeWord(addr uint16, x uint16) {
	m.writeByte(addr, uint8(x&0xFF))
	m.writeByte(addr+1, uint8(x>>8))
}

func (m *memory) readROM(addr uint16) byte {
	if addr < 0x4000 {
		return m.rom[addr]
	}
	return m.rom[int(addr)-0x4000+m.romBank*0x4000]
}

func (m *memory) writeROM(addr uint16, x byte) {
	switch {
	case addr >= 0x6000:
		switch m.mbcType {
		case mbc1:
			m.ramMode = x&1 == 1
			if m.ramMode {
				m.romBank &= 0x1F
			} else {
				m.eramBank = 0
			}
		case mbc3:
			// TODO RTC latch
		}
	case addr >= 0x4000:
		switch m.mbcType {
		case mbc1:
			if m.ramMode {
				m.eramBank = uint16(x) & 3
			} else {
				m.romBank &= 0x1F
				m.romBank |= (int(x) & 3) << 5
				m.romBank %= m.romBanks
			}
		case mbc3:
			if m.ramMode {
				m.eramBank = uint16(x) & 3
			} else {
				// TODO RTC register select
			}
		}
	case addr >= 0x2000:
		switch m.mbcType {
		case mbc1:
			x &= 0x1F
			if x == 0 {
				x++
			}
			m.romBank &= 0x60
			m.romBank |= int(x)
			m.romBank %= m.romBanks
		case mbc2:
			if addr&0x0100 != 0 {
				x &= 0x0F
				if x == 0 {
					x++
				}
				m.romBank = int(x)
			}
		case mbc3:
			x &= 0x7F
			if x == 0 {
				x++
			}
			m.romBank = int(x)
		}
	}
}

func (m *memory) readVideoRAM(addr uint16) byte {
	return m.vram[addr-0x8000]
}

func (m *memory) writeVideoRAM(addr uint16, x byte) {
	m.vram[addr-0x8000] = x
}

func (m *memory) readExternalRAM(addr uint16) byte {
	return m.eram[addr-0xA000+m.eramBank*0x2000]
}

func (m *memory) writeExternalRAM(addr uint16, x byte) {
	m.eram[addr-0xA000+m.eramBank*0x2000] = x
}

func (m *memory) readWorkRAM(addr uint16) byte {
	return m.wram[addr-0xC000]
}

func (m *memory) writeWorkRAM(addr uint16, x byte) {
	m.wram[addr-0xC000] = x
}

func (m *memory) readOAM(addr uint16) byte {
	return m.oam[addr-0xFE00]
}

func (m *memory) writeOAM(addr uint16, x byte) {
	m.oam[addr-0xFE00] = x
}

func (m *memory) readPort(addr uint16) byte {
	x := m.hram[addr-0xFF00]
	switch addr {
	case portNR52:
		x &= 0x80
		if m.audio.ch1.active {
			x |= 0x01
		}
		if m.audio.ch2.active {
			x |= 0x02
		}
		if m.audio.ch3.active {
			x |= 0x04
		}
		if m.audio.ch4.active {
			x |= 0x08
		}
	}
	return x
}

func (m *memory) writePort(addr uint16, x byte) {
	switch addr {
	case portJOYP:
		x &= 0x30
		switch x {
		case 0x00:
			x |= 0x0F
		case 0x10:
			x |= m.btnBits
		case 0x20:
			x |= m.dpadBits
		case 0x30:
			x |= 0x0F
		}
	case portSC:
		x = 0
	case portDIV:
		x = 0
	case portTAC:
		switch x & 3 {
		case 0:
			m.timaOverflow = 256
		case 1:
			m.timaOverflow = 4
		case 2:
			m.timaOverflow = 16
		case 3:
			m.timaOverflow = 64
		}
		m.timaTicks = 0
	case portNR10:
		m.audio.ch1.sweepTime = int(x>>4) & 3
		m.audio.ch1.sweepDir = 1
		if x&0x08 == 0x08 {
			m.audio.ch1.sweepDir = -1
		}
		m.audio.ch1.sweepShift = uint(x & 3)
	case portNR11:
		m.audio.ch1.waveDuty = int(x >> 6)
		m.audio.ch1.length = int(x & 0x3F)
	case portNR12:
		m.audio.ch1.volumeInit = int(x >> 4)
		m.audio.ch1.volumeDir = 1
		if x&0x08 == 0 {
			m.audio.ch1.volumeDir = -1
		}
		m.audio.ch1.volumeTime = int(x & 0x07)
		if m.audio.ch1.volumeInit == 0 {
			m.audio.ch1.volume = 0
		}
	case portNR13:
		freq := int(m.hram[portNR14-0xFF00]&0x07) << 8
		freq |= int(x)
		m.audio.ch1.freq = 131072 / (2048 - freq)
	case portNR14:
		m.audio.ch1.init = x&0x80 == 0x80
		m.audio.ch1.loop = x&0x40 == 0
		freq := int(m.hram[portNR13-0xFF00])
		freq |= int(x&0x07) << 8
		m.audio.ch1.freq = 131072 / (2048 - freq)
	case portNR21:
		m.audio.ch2.waveDuty = int(x >> 6)
		m.audio.ch2.length = int(x & 0x3F)
	case portNR22:
		m.audio.ch2.volumeInit = int(x >> 4)
		m.audio.ch2.volumeDir = 1
		if x&0x08 == 0 {
			m.audio.ch2.volumeDir = -1
		}
		m.audio.ch1.volumeTime = int(x & 0x07)
		if m.audio.ch2.volumeInit == 0 {
			m.audio.ch2.volume = 0
		}
	case portNR23:
		freq := int(m.hram[portNR24-0xFF00]&0x07) << 8
		freq |= int(x)
		m.audio.ch2.freq = 131072 / (2048 - freq)
	case portNR24:
		m.audio.ch2.init = x&0x80 == 0x80
		m.audio.ch2.loop = x&0x40 == 0
		freq := int(m.hram[portNR23-0xFF00])
		freq |= int(x&0x07) << 8
		m.audio.ch2.freq = 131072 / (2048 - freq)
	case portNR30:
		m.audio.ch3.on = x&0x80 == 0x80
	case portNR31:
		m.audio.ch3.length = int(x)
	case portNR32:
		m.audio.ch3.level = int(x>>5) & 0x03
	case portNR33:
		freq := int(m.hram[portNR34-0xFF00]&0x07) << 8
		freq |= int(x)
		m.audio.ch3.freq = 65536 / (2048 - freq)
	case portNR34:
		m.audio.ch3.init = x&0x80 == 0x80
		m.audio.ch3.loop = x&0x40 == 0
		freq := int(m.hram[portNR33-0xFF00])
		freq |= int(x&0x07) << 8
		m.audio.ch3.freq = 65536 / (2048 - freq)
	case portNR41:
		m.audio.ch4.length = int(x & 0x3F)
	case portNR42:
		m.audio.ch4.volumeInit = int(x >> 4)
		m.audio.ch4.volumeDir = 1
		if x&0x08 == 0 {
			m.audio.ch4.volumeDir = -1
		}
		m.audio.ch4.volumeTime = int(x & 0x07)
		if m.audio.ch4.volumeInit == 0 {
			m.audio.ch4.volume = 0
		}
	case portNR43:
		m.audio.ch4.shiftClockFreq = uint(x >> 4)
		m.audio.ch4.counterStepWidth = 15
		if x&0x08 == 0x08 {
			m.audio.ch4.counterStepWidth = 7
		}
		m.audio.ch4.dividingRatio = int(x & 0x07)
	case portNR44:
		m.audio.ch4.init = x&0x80 == 0x80
		m.audio.ch4.loop = x&0x40 == 0
	case portNR50:
		//m.audio.vinL = x&0x80 == 0x80
		m.audio.volL = int16(x&0x70) >> 4
		//m.audio.vinR = x&0x08 == 0x08
		m.audio.volR = int16(x & 0x07)
	case portNR51:
		m.audio.ch1L = int16(x) & 1
		m.audio.ch2L = int16(x>>1) & 1
		m.audio.ch3L = int16(x>>2) & 1
		m.audio.ch4L = int16(x>>3) & 1
		m.audio.ch1R = int16(x>>4) & 1
		m.audio.ch2R = int16(x>>5) & 1
		m.audio.ch3R = int16(x>>6) & 1
		m.audio.ch4R = int16(x>>7) & 1
	case portNR52:
		enable := x&0x80 == 0x80
		if enable != m.audio.enable {
			m.audio.pause(!enable)
		}
	case portLCDC:
		m.lcd.enable = x&0x80 != 0
		m.lcd.windowMap = x&0x40 != 0
		m.lcd.windowEnable = x&0x20 != 0
		m.lcd.tileData = x&0x10 != 0
		m.lcd.bgMap = x&0x08 != 0
		m.lcd.spriteSize = x&0x04 != 0
		m.lcd.spriteEnable = x&0x02 != 0
		m.lcd.bgEnable = x&0x01 != 0
	case portSTAT:
		m.lcd.lycInterrupt = x&0x40 != 0
		m.lcd.oamInterrupt = x&0x20 != 0
		m.lcd.vblankInterrupt = x&0x10 != 0
		m.lcd.hblankInterrupt = x&0x08 != 0
	case portLY:
		m.lcd.ly = 0
		m.lcd.clock = 0
		x = 0
	case portSCY:
		m.lcd.scy = x
	case portSCX:
		m.lcd.scx = x
	case portWY:
		m.lcd.wy = x
	case portWX:
		m.lcd.wx = x
	case portBGP:
		m.lcd.bgp[0] = x & 3
		m.lcd.bgp[1] = (x >> 2) & 3
		m.lcd.bgp[2] = (x >> 4) & 3
		m.lcd.bgp[3] = (x >> 6)
	case portOBP0:
		m.lcd.obp[0][0] = x & 3
		m.lcd.obp[0][1] = (x >> 2) & 3
		m.lcd.obp[0][2] = (x >> 4) & 3
		m.lcd.obp[0][3] = x >> 6
	case portOBP1:
		m.lcd.obp[1][0] = x & 3
		m.lcd.obp[1][1] = (x >> 2) & 3
		m.lcd.obp[1][2] = (x >> 4) & 3
		m.lcd.obp[1][3] = x >> 6
	case portDMA:
		src := uint16(x) << 8
		m.dma(src)
	}
	m.hram[addr-0xFF00] = x
}

func (m *memory) updateTimers(t int) {
	m.divTicks += t
	if m.divTicks > divOverflow {
		d := m.divTicks / divOverflow
		m.hram[0x04] += byte(d)
		m.divTicks -= d * divOverflow
	}
	if m.hram[0x07]&4 == 0 {
		return
	}
	m.timaTicks += t
	if m.timaTicks > m.timaOverflow {
		d := m.timaTicks / m.timaOverflow
		x := int(m.hram[0x05]) + d
		if x > 0xFF {
			m.hram[0x05] = m.hram[0x06] + byte(x)
			m.hram[0x0F] |= 0x04
		} else {
			m.hram[0x05] = byte(x)
		}
		m.timaTicks -= d * m.timaOverflow
	}
}

func (m *memory) dma(src uint16) {
	for i := 0; i < 0xA0; i++ {
		m.oam[i] = m.readByte(src)
		src++
	}
}

func (m *memory) monitorEvents() {
	for {
		event := <-sdl.Events
		switch ev := event.(type) {
		case sdl.QuitEvent:
			m.quit = true
		case sdl.KeyboardEvent:
			m.updateKeys(&ev)
		case sdl.JoyAxisEvent:
			m.updateDPad(&ev)
		case sdl.JoyButtonEvent:
			m.updateButtons(&ev)
		}
	}
}

func (m *memory) updateKeys(ev *sdl.KeyboardEvent) {
	// TODO trigger interrupts
	switch ev.Type {
	case sdl.KEYUP:
		switch ev.Keysym.Sym {
		case sdl.K_DOWN:
			m.dpadBits |= 0x08
		case sdl.K_UP:
			m.dpadBits |= 0x04
		case sdl.K_LEFT:
			m.dpadBits |= 0x02
		case sdl.K_RIGHT:
			m.dpadBits |= 0x01
		case sdl.K_RETURN:
			m.btnBits |= 0x08
		case sdl.K_RSHIFT:
			m.btnBits |= 0x04
		case sdl.K_z:
			m.btnBits |= 0x02
		case sdl.K_x:
			m.btnBits |= 0x01
		}
	case sdl.KEYDOWN:
		switch ev.Keysym.Sym {
		case sdl.K_DOWN:
			m.dpadBits &^= 0x08
		case sdl.K_UP:
			m.dpadBits &^= 0x04
		case sdl.K_LEFT:
			m.dpadBits &^= 0x02
		case sdl.K_RIGHT:
			m.dpadBits &^= 0x01
		case sdl.K_RETURN:
			m.btnBits &^= 0x08
		case sdl.K_RSHIFT:
			m.btnBits &^= 0x04
		case sdl.K_z:
			m.btnBits &^= 0x02
		case sdl.K_x:
			m.btnBits &^= 0x01
		}
	}
}

func (m *memory) updateDPad(ev *sdl.JoyAxisEvent) {
	switch int(ev.Axis) {
	case m.config.JoyAxisX:
		switch {
		case ev.Value > 3200:
			m.dpadBits &^= 0x01
			m.dpadBits |= 0x02
		case ev.Value < -3200:
			m.dpadBits |= 0x01
			m.dpadBits &^= 0x02
		default:
			m.dpadBits |= 0x03
		}
	case m.config.JoyAxisY:
		switch {
		case ev.Value > 3200:
			m.dpadBits |= 0x04
			m.dpadBits &^= 0x08
		case ev.Value < -3200:
			m.dpadBits &^= 0x04
			m.dpadBits |= 0x08
		default:
			m.dpadBits |= 0x0C
		}
	}
}

func (m *memory) updateButtons(ev *sdl.JoyButtonEvent) {
	switch ev.Type {
	case sdl.JOYBUTTONUP:
		switch int(ev.Button) {
		case m.config.JoyButtonA:
			m.btnBits |= 0x01
		case m.config.JoyButtonB:
			m.btnBits |= 0x02
		case m.config.JoyButtonSelect:
			m.btnBits |= 0x04
		case m.config.JoyButtonStart:
			m.btnBits |= 0x08
		}
	case sdl.JOYBUTTONDOWN:
		switch int(ev.Button) {
		case m.config.JoyButtonA:
			m.btnBits &^= 0x01
		case m.config.JoyButtonB:
			m.btnBits &^= 0x02
		case m.config.JoyButtonSelect:
			m.btnBits &^= 0x04
		case m.config.JoyButtonStart:
			m.btnBits &^= 0x08
		}
	}
}

func (m *memory) save(dir string) os.Error {
	if !m.rom.hasBattery() {
		return nil
	}

	name := m.saveName()
	file := path.Join(dir, name)
	size := m.rom.ramSize()
	switch m.mbcType {
	case mbc2:
		return ioutil.WriteFile(file, m.eram[0:512], 0644)
	case mbc3:
		if size == 0 {
			return nil
		}
		fallthrough
	default:
		return ioutil.WriteFile(file, m.eram[0:size], 0644)
	}
	return nil
}

func (m *memory) load(dir string) interface{} {
	if !m.rom.hasBattery() {
		return nil
	}

	name := m.saveName()
	file := path.Join(dir, name)
	size := m.rom.ramSize()

	if m.mbcType == mbc3 && size == 0 {
		return nil
	} else if m.mbcType == mbc2 {
		size = 512
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	if len(data) != size {
		return fmt.Sprintf("save should be %d bytes (%d found)",
			size, len(data))
	}

	for i := 0; i < size; i++ {
		m.eram[i] = data[i]
	}

	return nil
}

func (m *memory) saveName() string {
	return fmt.Sprintf("%s-%02X-%04X.battery",
		m.rom.title(), m.rom.headerChecksum(), m.rom.globalChecksum())
}

func (m *memory) dump(w io.Writer) {
	var addr int

	read := func() (x byte, e interface{}) {
		defer func() {
			e = recover()
			addr++
		}()
		return m.readByte(uint16(addr)), e
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
		fmt.Fprintln(w, "|")
	}
	fmt.Fprintln(w)
}
