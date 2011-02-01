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
	tilesX = 20
	tilesY = 18
	tileW = 8
	tileH = 8
	mapW = 32
	mapH = 32
	lcdW = tileW * tilesX
	lcdH = tileH * tilesY

	scale = 3

	screenW = lcdW * scale
	screenH = lcdH * scale

	hblankTicks = 204/4
	vblankTicks = 4560/4
	oamTicks    = 80/4
	vramTicks   = 172/4

	scanlineTicks = oamTicks + vramTicks + hblankTicks
	refreshTicks  = scanlineTicks * screenH + vblankTicks
)

type lcd struct {
	*memory
	*sdl.Surface
	clock int
	pal []uint32
	frameTime int64
}

func newLCD(m *memory) *lcd {
	screen := sdl.SetVideoMode(screenW, screenH, 0, sdl.DOUBLEBUF)
	pal := make([]uint32, 4)
	pal[0] = sdl.MapRGBA(screen.Format, 0x9B, 0xBC, 0x0F, 0)
	pal[1] = sdl.MapRGBA(screen.Format, 0x8B, 0xAC, 0x0F, 0)
	pal[2] = sdl.MapRGBA(screen.Format, 0x30, 0x62, 0x30, 0)
	pal[3] = sdl.MapRGBA(screen.Format, 0x0F, 0x38, 0x0F, 0)
	screen.FillRect(nil, pal[0])
	screen.Flip()
	time := time.Nanoseconds()
	return &lcd{memory: m, Surface: screen,	pal: pal, frameTime: time}
}

func (scr *lcd) step(t int) {
	scr.clock += t
	if scr.clock >= refreshTicks {
		scr.clock -= refreshTicks
	}

	ly := byte(scr.clock / scanlineTicks)
	scr.writePort(portLY, ly)
	mode := calcMode(scr.clock, ly)
	if mode == scr.lcdMode {
		return;
	}
	scr.lcdMode = mode

	stat := scr.readPort(portSTAT) &^ 3 | mode
	irq := scr.readPort(portIF)

	switch mode {
	case modeOAM:
		if scr.oamInterrupt {
			scr.writePort(portIF, irq | 0x02)
		}
		if lyc := scr.readPort(portLYC); ly-1 == lyc {
			stat |= 0x04
			if scr.lycInterrupt {
				scr.writePort(portIF, irq | 0x02)
			}
		} else {
			stat &^= 0x04
		}
	case modeVRAM:
		if scr.lcdEnable {
			scr.scanline(ly)
		}
	case modeHBlank:
		if scr.hblankInterrupt {
			scr.writePort(portIF, irq | 0x02)
		}
	case modeVBlank:
		if scr.vblankInterrupt {
			irq |= 0x02
		}
		scr.writePort(portIF, irq | 0x01)
		if !scr.lcdEnable {
			scr.FillRect(nil, scr.pal[0])
		}
		scr.Flip()
		scr.delay()
		scr.pumpEvents()
	}

	scr.writePort(portSTAT, stat)
}

func (scr *lcd) delay() {
	now := time.Nanoseconds()
	delta := now - scr.frameTime
	target := 16742706 - delta
	if target > 0 {
		time.Sleep(target)
	}
	scr.frameTime = time.Nanoseconds()
}

func (scr *lcd) scanline(ly byte) {
	scy := scr.readPort(portSCY)
	scx := scr.readPort(portSCX)
	wy := scr.readPort(portWY)
	wx := scr.readPort(portWX)

	if scr.bgEnable {
		scr.rleline(scr.bgMap, byte(0), ly, scx, scy)
	}

	if scr.windowEnable && wx < 167 && wy < 144 && ly >= wy {
		x := int(wx) - 7
		xoff := -x
		if x < 0 { x = 0 }
		scr.rleline(scr.windowMap, byte(x), ly, byte(xoff), byte(-wy))
	}

	if scr.spriteEnable {
		count := 0
		idx := 0
		// TODO sprite priorities for overlapping sprites at
		// different x-coordinates (lower x-coordinate wins)
		for idx < 0xA0 && count < 10 {
			y := int(scr.oam[idx]) - 16; idx++
			x := int(scr.oam[idx]) - 8; idx++
			tile := int(scr.oam[idx]); idx++
			info := scr.oam[idx]; idx++
			h := 8
			if scr.spriteSize { h = 16; tile &= 0xFE }
			if int(ly) < y || int(ly) >= y + h { continue }
			count++
			if x == -8 || x >= 168 { continue }
			masked := info & 0x80 == 0x80
			yflip := info & 0x40 == 0x40
			xflip := info & 0x20 == 0x20
			palidx := (info >> 4) & 1
			tiley := int(ly) - y
			if yflip { tiley = h - tiley }
			tile = tile * 16 + tiley * 2
			rect := &sdl.Rect{Y:int16(ly)*scale, W:scale, H:scale}
			for i := 0; i < 8; i++ {
				xi := x + i
				if xi < 0 { continue }
				if xi >= screenW { continue }
				if masked {
					px := scr.mapAt(scr.bgMap,
						xi + int(scx),
						int(ly) + int(scy))
					if px != 0 ||
						xi > int(wx) ||
						ly >= wy { continue }
				}
				bit := uint(i)
				if !xflip { bit = uint(7 - i) }
				px := (scr.vram[tile] >> bit) & 1
				px |= ((scr.vram[tile+1] >> bit) & 1) << 1
				if px != 0 {
					color := scr.pal[scr.obp[palidx][px]]
					rect.X = int16(xi) * scale
					scr.FillRect(rect, color)
				}
			}
		}
	}
}

func (scr *lcd) rleline(map1 bool, x, y, xoff, yoff byte) {
	r := &sdl.Rect{ X: int16(x) * scale,
		Y: int16(y) * scale, W: scale, H: scale }
	y += yoff
	cur := scr.mapAt(map1, int(x + xoff), int(y))
	for x++; x < lcdW; x++ {
		b := scr.mapAt(map1, int(x + xoff), int(y))
		if b != cur {
			scr.FillRect(r, scr.pal[scr.bgp[cur]])
			cur = b
			r.X = int16(x) * scale
			r.W = scale
		} else {
			r.W += scale
		}
	}
	scr.FillRect(r, scr.pal[scr.bgp[cur]])
}

func (scr *lcd) mapAt(map1 bool, x, y int) byte {
	idx := (y / tileH) * mapW + x / tileW
	if map1 {
		idx += 0x1C00
	} else {
		idx += 0x1800
	}
	tile := scr.vram[idx]
	if scr.tileData {
		idx = int(tile) * 16
	} else {
		idx = 0x1000 + int(int8(tile)) * 16
	}
	idx += (y % tileH) * 2
	bit := uint(tileW - 1 - x % tileW)
	px := (scr.vram[idx] >> bit) & 1
	px |= ((scr.vram[idx+1] >> bit) & 1) << 1
	return px
}

func calcMode(t int, ly byte) byte {
	if ly >= lcdH {
		return modeVBlank
	}
	t %= scanlineTicks
	if t < oamTicks {
		return modeOAM
	}
	if t < oamTicks + vramTicks {
		return modeVRAM
	}
	return modeHBlank
}
