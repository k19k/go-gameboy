// Copyright 2011 Kevin Bulusek. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gameboy

import (
	"⚛sdl"
	"time"
)

const (
	modeHBlank = byte(iota)
	modeVBlank
	modeOAM
	modeVRAM
)

const (
	tilesX   = 20
	tilesY   = 18
	tileW    = 8
	tileH    = 8
	mapW     = 32
	mapH     = 32
	displayW = tileW * tilesX
	displayH = tileH * tilesY

	hblankTicks = 204 / 4
	vblankTicks = 4560 / 4
	oamTicks    = 80 / 4
	vramTicks   = 172 / 4

	scanlineTicks = oamTicks + vramTicks + hblankTicks
	refreshTicks  = scanlineTicks*displayH + vblankTicks
)

type display struct {
	*memory
	*sdl.Surface
	pal       [4]uint32
	frameTime int64
	screenW   int
	screenH   int

	clock int

	// LCDC flags
	enable       bool
	windowMap    bool
	windowEnable bool
	tileData     bool
	bgMap        bool
	spriteSize   bool
	spriteEnable bool
	bgEnable     bool

	// STAT flags
	lycInterrupt    bool
	oamInterrupt    bool
	vblankInterrupt bool
	hblankInterrupt bool
	mode            byte

	// LCD registers
	ly       byte
	scy, scx byte
	wy, wx   byte

	bgp [4]byte
	obp [2][4]byte

	// Scanlines are rendered to here first, and then drawn to the
	// display - rather than each layer accessing the display
	// separately (which is slow).
	lineBuf [displayW]byte

	// When rendering a scanline this is zeroed out, then
	// logical-ORed with the pixels from the BG and window. This
	// is then used to lookup which pixels can be painted in
	// sprites that are to be obscured by the background layers.
	oamLineMask [displayW]byte
}

func newDisplay(m *memory) *display {
	sdl.WM_SetCaption(m.rom.title(), "")
	lcd := display{memory: m}
	lcd.screenW = displayW * m.config.Scale
	lcd.screenH = displayH * m.config.Scale
	lcd.Surface = sdl.SetVideoMode(lcd.screenW, lcd.screenH, 0,
		sdl.DOUBLEBUF)
	lcd.pal[0] = sdl.MapRGBA(lcd.Format, 0x9B, 0xBC, 0x0F, 0)
	lcd.pal[1] = sdl.MapRGBA(lcd.Format, 0x8B, 0xAC, 0x0F, 0)
	lcd.pal[2] = sdl.MapRGBA(lcd.Format, 0x30, 0x62, 0x30, 0)
	lcd.pal[3] = sdl.MapRGBA(lcd.Format, 0x0F, 0x38, 0x0F, 0)
	sdl.ShowCursor(sdl.DISABLE)
	lcd.FillRect(nil, lcd.pal[0])
	lcd.Flip()
	lcd.frameTime = time.Nanoseconds()
	return &lcd
}

func (lcd *display) step(t int) {
	lcd.clock += t
	if lcd.clock >= refreshTicks {
		lcd.clock -= refreshTicks
	}

	lcd.ly = byte(lcd.clock / scanlineTicks)
	lcd.hram[portLY-0xFF00] = lcd.ly
	mode := calcMode(lcd.clock, lcd.ly)
	if mode == lcd.mode {
		return
	}
	lcd.mode = mode

	stat := lcd.readPort(portSTAT)&^3 | mode
	irq := lcd.readPort(portIF)

	switch mode {
	case modeOAM:
		if lcd.oamInterrupt {
			lcd.writePort(portIF, irq|0x02)
		}
		if lyc := lcd.readPort(portLYC); lcd.ly-1 == lyc {
			stat |= 0x04
			if lcd.lycInterrupt {
				lcd.writePort(portIF, irq|0x02)
			}
		} else {
			stat &^= 0x04
		}
	case modeVRAM:
		if lcd.enable {
			lcd.scanline()
		}
	case modeHBlank:
		if lcd.hblankInterrupt {
			lcd.writePort(portIF, irq|0x02)
		}
	case modeVBlank:
		if lcd.vblankInterrupt {
			irq |= 0x02
		}
		lcd.writePort(portIF, irq|0x01)
		lcd.Flip()

		// while audio is playing, we let it control the
		// emulation speed
		if !lcd.audio.enable {
			lcd.delay()
		}
	}

	lcd.writePort(portSTAT, stat)
}

func (lcd *display) delay() {
	now := time.Nanoseconds()
	delta := now - lcd.frameTime
	target := 16742706 - delta
	if target > 0 {
		time.Sleep(target)
	}
	lcd.frameTime = time.Nanoseconds()
}

func (lcd *display) scanline() {
	for i := 0; i < displayW; i++ {
		lcd.oamLineMask[i] = 0
		lcd.lineBuf[i] = 0
	}

	if lcd.bgEnable {
		lcd.mapline(lcd.bgMap, byte(0), lcd.scx, lcd.scy)
	}

	if lcd.windowEnable {
		if lcd.wx < 167 && lcd.wy < 144 && lcd.ly >= lcd.wy {
			x := int(lcd.wx) - 7
			xoff := -x
			if x < 0 {
				x = 0
			}
			lcd.mapline(lcd.windowMap, byte(x),
				byte(xoff), byte(-lcd.wy))
		}
	}

	if lcd.spriteEnable {
		lcd.oamline()
	}

	lcd.flushline()
}

func (lcd *display) mapline(map1 bool, x, xoff, yoff byte) {
	y := lcd.ly + yoff
	for ; x < displayW; x++ {
		b := lcd.mapAt(map1, int(x+xoff), int(y))
		lcd.oamLineMask[x] |= b
		lcd.lineBuf[x] = lcd.bgp[b]
	}
}

func (lcd *display) flushline() {
	// Do some simple run-length counting to reduce the number of
	// FillRect calls we need to make.
	scale := uint16(lcd.config.Scale)
	r := &sdl.Rect{0, int16(lcd.ly) * int16(scale), scale, scale}
	cur := lcd.lineBuf[0]
	for x := 1; x < displayW; x++ {
		b := lcd.lineBuf[x]
		if b != cur {
			lcd.FillRect(r, lcd.pal[cur])
			cur = b
			r.X = int16(x) * int16(scale)
			r.W = scale
		} else {
			r.W += scale
		}
	}
	lcd.FillRect(r, lcd.pal[cur])
}

// oamline draws up to 10 sprites on the current scanline
func (lcd *display) oamline() {
	// TODO sprite priorities for overlapping sprites at
	// different x-coordinates (lower x-coordinate wins)

	count := 0
	for idx := 0; idx < 0xA0 && count < 10; idx += 4 {
		y := int(lcd.oam[idx]) - 16
		x := int(lcd.oam[idx+1]) - 8
		tile := int(lcd.oam[idx+2])
		info := lcd.oam[idx+3]

		h := 8
		if lcd.spriteSize {
			h = 16
			tile &= 0xFE
		}

		if int(lcd.ly) < y || int(lcd.ly) >= y+h {
			continue
		}
		count++
		if x == -8 || x >= 168 {
			continue
		}

		lcd.spriteLine(tile, x, y, h, info)
	}
}

func (lcd *display) spriteLine(tile, x, y, h int, info byte) {
	masked := info&0x80 == 0x80
	yflip := info&0x40 == 0x40
	xflip := info&0x20 == 0x20
	palidx := (info >> 4) & 1

	tiley := int(lcd.ly) - y
	if yflip {
		tiley = h - 1 - tiley
	}
	tile = tile*16 + tiley*2

	for i := 0; i < 8; i++ {
		xi := byte(x + i)
		if xi >= displayW {
			if xi > 248 { // i.e., xi < 0
				continue
			}
			return
		}
		if masked && lcd.oamLineMask[xi] != 0 {
			continue
		}
		bit := uint(i)
		if !xflip {
			bit = uint(7 - i)
		}
		px := (lcd.vram[tile] >> bit) & 1
		px |= ((lcd.vram[tile+1] >> bit) & 1) << 1
		if px != 0 {
			lcd.lineBuf[xi] = lcd.obp[palidx][px]
		}
	}
}

func (lcd *display) mapAt(map1 bool, x, y int) byte {
	idx := (y/tileH)*mapW + x/tileW
	if map1 {
		idx += 0x1C00
	} else {
		idx += 0x1800
	}
	tile := lcd.vram[idx]
	if lcd.tileData {
		idx = int(tile) * 16
	} else {
		idx = 0x1000 + int(int8(tile))*16
	}
	idx += (y % tileH) * 2
	bit := uint(tileW - 1 - x%tileW)
	px := (lcd.vram[idx] >> bit) & 1
	px |= ((lcd.vram[idx+1] >> bit) & 1) << 1
	return px
}

func calcMode(t int, ly byte) byte {
	if ly >= displayH {
		return modeVBlank
	}
	t %= scanlineTicks
	if t < oamTicks {
		return modeOAM
	}
	if t < oamTicks+vramTicks {
		return modeVRAM
	}
	return modeHBlank
}
