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

	scale = 3

	screenW = displayW * scale
	screenH = displayH * scale

	hblankTicks = 204 / 4
	vblankTicks = 4560 / 4
	oamTicks    = 80 / 4
	vramTicks   = 172 / 4

	scanlineTicks = oamTicks + vramTicks + hblankTicks
	refreshTicks  = scanlineTicks*screenH + vblankTicks
)

type display struct {
	*memory
	*sdl.Surface
	pal       []uint32
	frameTime int64
}

func newDisplay(m *memory) *display {
	screen := sdl.SetVideoMode(screenW, screenH, 0, sdl.DOUBLEBUF)
	pal := make([]uint32, 4)
	pal[0] = sdl.MapRGBA(screen.Format, 0x9B, 0xBC, 0x0F, 0)
	pal[1] = sdl.MapRGBA(screen.Format, 0x8B, 0xAC, 0x0F, 0)
	pal[2] = sdl.MapRGBA(screen.Format, 0x30, 0x62, 0x30, 0)
	pal[3] = sdl.MapRGBA(screen.Format, 0x0F, 0x38, 0x0F, 0)
	screen.FillRect(nil, pal[0])
	screen.Flip()
	time := time.Nanoseconds()
	return &display{memory: m, Surface: screen, pal: pal, frameTime: time}
}

func (lcd *display) step(t int) {
	lcd.clock += t
	if lcd.clock >= refreshTicks {
		lcd.clock -= refreshTicks
	}

	ly := byte(lcd.clock / scanlineTicks)
	lcd.hram[portLY-0xFF00] = ly
	mode := calcMode(lcd.clock, ly)
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
		if lyc := lcd.readPort(portLYC); ly-1 == lyc {
			stat |= 0x04
			if lcd.lycInterrupt {
				lcd.writePort(portIF, irq|0x02)
			}
		} else {
			stat &^= 0x04
		}
	case modeVRAM:
		if lcd.lcdEnable {
			lcd.scanline(ly)
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
		if !lcd.lcdEnable {
			lcd.FillRect(nil, lcd.pal[0])
		}
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

func (lcd *display) scanline(ly byte) {
	scy := lcd.readPort(portSCY)
	scx := lcd.readPort(portSCX)
	wy := lcd.readPort(portWY)
	wx := lcd.readPort(portWX)

	if lcd.bgEnable {
		lcd.rleline(lcd.bgMap, byte(0), ly, scx, scy)
	}

	if lcd.windowEnable && wx < 167 && wy < 144 && ly >= wy {
		x := int(wx) - 7
		xoff := -x
		if x < 0 {
			x = 0
		}
		lcd.rleline(lcd.windowMap, byte(x), ly, byte(xoff), byte(-wy))
	}

	if lcd.spriteEnable {
		count := 0
		idx := 0
		// TODO sprite priorities for overlapping sprites at
		// different x-coordinates (lower x-coordinate wins)
		for idx < 0xA0 && count < 10 {
			y := int(lcd.oam[idx]) - 16
			idx++
			x := int(lcd.oam[idx]) - 8
			idx++
			tile := int(lcd.oam[idx])
			idx++
			info := lcd.oam[idx]
			idx++
			h := 8
			if lcd.spriteSize {
				h = 16
				tile &= 0xFE
			}
			if int(ly) < y || int(ly) >= y+h {
				continue
			}
			count++
			if x == -8 || x >= 168 {
				continue
			}
			masked := info&0x80 == 0x80
			yflip := info&0x40 == 0x40
			xflip := info&0x20 == 0x20
			palidx := (info >> 4) & 1
			tiley := int(ly) - y
			if yflip {
				tiley = h - 1 - tiley
			}
			tile = tile*16 + tiley*2
			rect := &sdl.Rect{Y: int16(ly) * scale,
				W: scale, H: scale}
			for i := 0; i < 8; i++ {
				xi := x + i
				if xi < 0 {
					continue
				}
				if xi >= screenW {
					continue
				}
				if masked {
					px := lcd.mapAt(lcd.bgMap,
						xi+int(scx),
						int(ly)+int(scy))
					if px != 0 ||
						xi > int(wx) ||
						ly >= wy {
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
					rect.X = int16(xi) * scale
					lcd.FillRect(rect, color)
				}
			}
		}
	}
}

func (lcd *display) rleline(map1 bool, x, y, xoff, yoff byte) {
	r := &sdl.Rect{X: int16(x) * scale,
		Y: int16(y) * scale, W: scale, H: scale}
	y += yoff
	cur := lcd.mapAt(map1, int(x+xoff), int(y))
	for x++; x < displayW; x++ {
		b := lcd.mapAt(map1, int(x+xoff), int(y))
		if b != cur {
			lcd.FillRect(r, lcd.pal[lcd.bgp[cur]])
			cur = b
			r.X = int16(x) * scale
			r.W = scale
		} else {
			r.W += scale
		}
	}
	lcd.FillRect(r, lcd.pal[lcd.bgp[cur]])
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
