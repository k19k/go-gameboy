package gameboy

import (
	"sdl"
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
	if mode == lcd.lcdMode {
		return
	}
	lcd.lcdMode = mode

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
		if lcd.lcdEnable {
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
		lcd.delay()
		lcd.pumpEvents()
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
	if lcd.bgEnable {
		lcd.rleline(lcd.bgMap, byte(0), lcd.scx, lcd.scy)
	}

	if lcd.windowEnable {
		if lcd.wx < 167 && lcd.wy < 144 && lcd.ly >= lcd.wy {
			x := int(lcd.wx) - 7
			xoff := -x
			if x < 0 {
				x = 0
			}
			lcd.rleline(lcd.windowMap, byte(x),
				byte(xoff), byte(-lcd.wy))
		}
	}

	if lcd.spriteEnable {
		lcd.oamline()
	}
}

func (lcd *display) rleline(map1 bool, x, xoff, yoff byte) {
	scale := int16(lcd.config.Scale)
	r := &sdl.Rect{X: int16(x) * scale, Y: int16(lcd.ly) * scale,
		W: uint16(scale), H: uint16(scale)}
	y := lcd.ly + yoff
	cur := lcd.mapAt(map1, int(x+xoff), int(y))
	for x++; x < displayW; x++ {
		b := lcd.mapAt(map1, int(x+xoff), int(y))
		if b != cur {
			lcd.FillRect(r, lcd.pal[lcd.bgp[cur]])
			cur = b
			r.X = int16(x) * scale
			r.W = uint16(scale)
		} else {
			r.W += uint16(scale)
		}
	}
	lcd.FillRect(r, lcd.pal[lcd.bgp[cur]])
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
	hidden := info&0x80 == 0x80
	yflip := info&0x40 == 0x40
	xflip := info&0x20 == 0x20
	palidx := (info >> 4) & 1

	tiley := int(lcd.ly) - y
	if yflip {
		tiley = h - 1 - tiley
	}
	tile = tile*16 + tiley*2

	scale := lcd.config.Scale
	rect := &sdl.Rect{Y: int16(lcd.ly) * int16(scale),
		W: uint16(scale), H: uint16(scale)}
	for i := 0; i < 8; i++ {
		xi := byte(x + i)
		if xi > 248 { // xi < 0
			continue
		}
		if xi >= displayW {
			return
		}
		if hidden {
			px := lcd.mapAt(lcd.bgMap,
				int(xi+lcd.scx), int(lcd.ly+lcd.scy))
			if px != 0 || xi > lcd.wx || lcd.ly >= lcd.wy {
				continue
			}
		}
		bit := uint(i)
		if !xflip {
			bit = uint(7 - i)
		}
		px := (lcd.vram[tile] >> bit) & 1
		px |= ((lcd.vram[tile+1] >> bit) & 1) << 1
		if px != 0 {
			color := lcd.pal[lcd.obp[palidx][px]]
			rect.X = int16(xi) * int16(scale)
			lcd.FillRect(rect, color)
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
