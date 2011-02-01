package main

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

type GPU struct {
	clock int
	pal []uint32
	screen *sdl.Surface
	mmu *MBC
	frameTime int64
}

func NewGPU(m *MBC) *GPU {
	screen := sdl.SetVideoMode(screenW, screenH, 0, sdl.DOUBLEBUF)
	pal := make([]uint32, 4)
	pal[0] = sdl.MapRGBA(screen.Format, 0x9B, 0xBC, 0x0F, 0)
	pal[1] = sdl.MapRGBA(screen.Format, 0x8B, 0xAC, 0x0F, 0)
	pal[2] = sdl.MapRGBA(screen.Format, 0x30, 0x62, 0x30, 0)
	pal[3] = sdl.MapRGBA(screen.Format, 0x0F, 0x38, 0x0F, 0)
	screen.FillRect(nil, pal[0])
	screen.Flip()
	time := time.Nanoseconds()
	return &GPU{pal: pal, screen: screen, mmu: m, frameTime: time}
}

func (gpu *GPU) Step(t int) {
	gpu.clock += t
	if gpu.clock >= refreshTicks {
		gpu.clock -= refreshTicks
	}

	mem := gpu.mmu
	ly := byte(gpu.clock / scanlineTicks)
	mem.WritePort(PortLY, ly)
	mode := calcMode(gpu.clock, ly)
	if mode == mem.LCDMode {
		return;
	}
	mem.LCDMode = mode

	stat := mem.ReadPort(PortSTAT) &^ 3 | mode
	irq := mem.ReadPort(PortIF)

	switch mode {
	case modeOAM:
		if mem.OAMInterrupt {
			mem.WritePort(PortIF, irq | 0x02)
		}
		if lyc := mem.ReadPort(PortLYC); ly-1 == lyc {
			stat |= 0x04
			if mem.LYCInterrupt {
				mem.WritePort(PortIF, irq | 0x02)
			}
		} else {
			stat &^= 0x04
		}
	case modeVRAM:
		if mem.LCDEnable {
			gpu.scanline(ly)
		}
	case modeHBlank:
		if mem.HBlankInterrupt {
			mem.WritePort(PortIF, irq | 0x02)
		}
	case modeVBlank:
		if mem.VBlankInterrupt {
			irq |= 0x02
		}
		mem.WritePort(PortIF, irq | 0x01)
		if !mem.LCDEnable {
			gpu.screen.FillRect(nil, gpu.pal[0])
		}
		gpu.screen.Flip()
		gpu.delay()
		mem.PumpEvents()
	}

	mem.WritePort(PortSTAT, stat)
}

func (gpu *GPU) delay() {
	now := time.Nanoseconds()
	delta := now - gpu.frameTime
	target := 16742706 - delta
	if target > 0 {
		time.Sleep(target)
	}
	gpu.frameTime = time.Nanoseconds()
}

func (gpu *GPU) scanline(ly byte) {
	mem := gpu.mmu

	scy := mem.ReadPort(PortSCY)
	scx := mem.ReadPort(PortSCX)
	wy := mem.ReadPort(PortWY)
	wx := mem.ReadPort(PortWX)

	if mem.BGEnable {
		gpu.rleline(mem.BGMap, byte(0), ly, scx, scy)
	}

	if mem.WindowEnable && wx < 167 && wy < 144 && ly >= wy {
		x := int(wx) - 7
		xoff := -x
		if x < 0 { x = 0 }
		gpu.rleline(mem.WindowMap, byte(x), ly, byte(xoff), byte(-wy))
	}

	if mem.SpriteEnable {
		count := 0
		idx := 0
		// TODO sprite priorities for overlapping sprites at
		// different x-coordinates (lower x-coordinate wins)
		for idx < 0xA0 && count < 10 {
			y := int(mem.oam[idx]) - 16; idx++
			x := int(mem.oam[idx]) - 8; idx++
			tile := int(mem.oam[idx]); idx++
			info := mem.oam[idx]; idx++
			h := 8
			if mem.SpriteSize { h = 16; tile &= 0xFE }
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
					px := gpu.mapAt(mem.BGMap,
						xi + int(scx),
						int(ly) + int(scy))
					if px != 0 ||
						xi > int(wx) ||
						ly >= wy { continue }
				}
				bit := uint(i)
				if !xflip { bit = uint(7 - i) }
				px := (mem.vram[tile] >> bit) & 1
				px |= ((mem.vram[tile+1] >> bit) & 1) << 1
				if px != 0 {
					color := gpu.pal[mem.OBP[palidx][px]]
					rect.X = int16(xi) * scale
					gpu.screen.FillRect(rect, color)
				}
			}
		}
	}
}

func (gpu *GPU) rleline(map1 bool, x, y, xoff, yoff byte) {
	r := &sdl.Rect{ X: int16(x) * scale,
		Y: int16(y) * scale, W: scale, H: scale }
	y += yoff
	cur := gpu.mapAt(map1, int(x + xoff), int(y))
	for x++; x < lcdW; x++ {
		b := gpu.mapAt(map1, int(x + xoff), int(y))
		if b != cur {
			gpu.screen.FillRect(r, gpu.pal[gpu.mmu.BGP[cur]])
			cur = b
			r.X = int16(x) * scale
			r.W = scale
		} else {
			r.W += scale
		}
	}
	gpu.screen.FillRect(r, gpu.pal[gpu.mmu.BGP[cur]])
}

func (gpu *GPU) mapAt(map1 bool, x, y int) byte {
	mem := gpu.mmu
	idx := (y / tileH) * mapW + x / tileW
	if map1 {
		idx += 0x1C00
	} else {
		idx += 0x1800
	}
	tile := mem.vram[idx]
	if mem.TileData {
		idx = int(tile) * 16
	} else {
		idx = 0x1000 + int(int8(tile)) * 16
	}
	idx += (y % tileH) * 2
	bit := uint(tileW - 1 - x % tileW)
	px := (mem.vram[idx] >> bit) & 1
	px |= ((mem.vram[idx+1] >> bit) & 1) << 1
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
