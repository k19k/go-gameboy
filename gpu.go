package main

import (
	"sdl"
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
	maps [4][mapW * mapH * 16]byte
	screen *sdl.Surface
	mmu *MBC
	frameSkip bool
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
	return &GPU{pal: pal, screen: screen, mmu: m}
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
		if mem.LCDEnable && !gpu.frameSkip {
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
		if !gpu.frameSkip {
			if !mem.LCDEnable {
				gpu.screen.FillRect(nil, gpu.pal[0])
			}
			gpu.screen.Flip()
		}
		gpu.frameSkip = !gpu.frameSkip
	}

	mem.WritePort(PortSTAT, stat)
}

func (gpu *GPU) scanline(ly byte) {
	mem := gpu.mmu

	if ly == 0 && mem.vramDirty {
		gpu.update()
	}

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
		// TODO
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
	idx := 0
	if map1 { idx = 2 }
	if !gpu.mmu.TileData { idx++ }
	n := ((y / tileH) * mapW + (x / tileW)) * 16
	n += (y % tileH) * 2
	b := uint(tileW - 1 - x % tileW)
	lo := (gpu.maps[idx][n] >> b) & 1
	hi := ((gpu.maps[idx][n+1] >> b) & 1) << 1
	return hi | lo
}

func (gpu *GPU) update() {
	mem := gpu.mmu
	for i := 0; i < 2048; i++ {
		tile := int(mem.vram[0x1800 + i])
		idx  := (i / 1024) * 2
		x := (i & 0x03FF) * 16 // (i % 1024) * 16
		if mem.mapDirty[i] || mem.tileDirty[tile] {
			src := tile * 16
			for n := 0; n < 16; n++ {
				gpu.maps[idx][x+n] = mem.vram[src+n]
			}
		}
		tile = int(int8(tile)) + 256
		idx++
		if mem.mapDirty[i] || mem.tileDirty[tile] {
			src := tile * 16
			for n := 0; n < 16; n++ {
				gpu.maps[idx][x+n] = mem.vram[src+n]
			}
		}
		mem.mapDirty[i] = false
	}
	for i := 0; i < 384; i++ {
		mem.tileDirty[i] = false
	}
	mem.vramDirty = false
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
