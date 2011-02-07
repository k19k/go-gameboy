// Copyright 2011 Kevin Bulusek. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gameboy

import (
	"bytes"
	"fmt"
	"io/ioutil"
)

const (
	mbcNone = iota
	mbc1
	mbc2
	mbc3
)

type romImage []byte

var nintendoLogo = []byte{
	0xCE, 0xED, 0x66, 0x66, 0xCC, 0x0D, 0x00, 0x0B,
	0x03, 0x73, 0x00, 0x83, 0x00, 0x0C, 0x00, 0x0D,
	0x00, 0x08, 0x11, 0x1F, 0x88, 0x89, 0x00, 0x0E,
	0xDC, 0xCC, 0x6E, 0xE6, 0xDD, 0xDD, 0xD9, 0x99,
	0xBB, 0xBB, 0x67, 0x63, 0x6E, 0x0E, 0xEC, 0xCC,
	0xDD, 0xDC, 0x99, 0x9F, 0xBB, 0xB9, 0x33, 0x3E,
}

func loadROM(path string) (rom []byte, err interface{}) {
	rom, err = ioutil.ReadFile(path)
	if err != nil {
		return
	}
	if len(rom) < 32768 {
		err = "invalid ROM image (size < 32kB)"
	}
	return
}

func (rom romImage) checkLogo() bool {
	return bytes.Compare(rom[0x0104:0x0134], nintendoLogo) == 0
}

func (rom romImage) title() string {
	raw := []byte(rom)
	for i := 0x0134; i < 0x0144; i++ {
		if rom[i] == 0 {
			return string(raw[0x0134:i])
		}
	}
	return string(raw[0x0134:0x0144])
}

func (rom romImage) mbcType() (mbc int, err interface{}) {
	switch rom[0x0147] {
	case 0x00:
		fallthrough
	case 0x08:
		fallthrough
	case 0x09:
		mbc = mbcNone
	case 0x01:
		fallthrough
	case 0x02:
		fallthrough
	case 0x03:
		mbc = mbc1
	case 0x05:
		fallthrough
	case 0x06:
		mbc = mbc2
	case 0x10:
		fallthrough
	case 0x11:
		fallthrough
	case 0x12:
		fallthrough
	case 0x13:
		mbc = mbc3
	default:
		err = fmt.Sprintf("unknown memory bank controller type (%02Xh)",
			rom[0x0147])
	}
	return
}

func (rom romImage) hasRAM() bool {
	switch rom[0x0147] {
	case 0x02:
		fallthrough
	case 0x03:
		fallthrough
	case 0x05:
		fallthrough
	case 0x06:
		fallthrough
	case 0x08:
		fallthrough
	case 0x09:
		fallthrough
	case 0x10:
		fallthrough
	case 0x12:
		fallthrough
	case 0x13:
		return true
	}
	return false
}

func (rom romImage) hasBattery() bool {
	switch rom[0x0147] {
	case 0x03:
		fallthrough
	case 0x06:
		fallthrough
	case 0x09:
		fallthrough
	case 0x0F:
		fallthrough
	case 0x10:
		fallthrough
	case 0x13:
		return true
	}
	return false
}

func (rom romImage) banks() (n int, err interface{}) {
	switch rom[0x0148] {
	case 0x00:
		n = 0
	case 0x01:
		n = 4
	case 0x02:
		n = 8
	case 0x03:
		n = 16
	case 0x04:
		n = 32
	case 0x05:
		n = 64
	case 0x06:
		n = 128
	case 0x08:
		n = 256
	case 0x52:
		n = 72
	case 0x53:
		n = 80
	case 0x54:
		n = 96
	default:
		err = fmt.Sprintf("invalid ROM banks (%02Xh)", rom[0x0148])
	}
	return
}

func (rom romImage) ramSize() int {
	switch rom[0x0149] & 3 {
	case 1:
		return 2048
	case 2:
		return 8192
	case 3:
		return 32768
	}
	return 0
}

func (rom romImage) headerChecksum() byte {
	var x byte
	for i := 0x0134; i < 0x014D; i++ {
		x = x - rom[i] - 1
	}
	return x
}

func (rom romImage) doHeaderChecksum() bool {
	return rom.headerChecksum() == rom[0x014D]
}

func (rom romImage) globalChecksum() uint16 {
	var x uint16
	for i, b := range rom {
		if i != 0x014E && i != 0x014F {
			x += uint16(b)
		}
	}
	return x
}

func (rom romImage) doGlobalChecksum() bool {
	x := rom.globalChecksum()
	return byte(x>>8) == rom[0x014E] &&
		byte(x) == rom[0x014F]
}
