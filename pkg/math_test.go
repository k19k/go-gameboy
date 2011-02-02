// Copyright 2011 Kevin Bulusek. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gameboy

import (
	"testing"
	"testing/quick"
)

func TestINC(t *testing.T) {
	f := func(x byte, fz, fn, fh bool) bool {
		sys := &cpu{fz: fz, fn: fn, fh: fh}
		y := sys.inc(x)
		return y == x+1 &&
			sys.fz == (y == 0) &&
			!sys.fn &&
			sys.fh == (x&0x0F == 0x0F)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestDEC(t *testing.T) {
	f := func(x byte, fz, fn, fh bool) bool {
		sys := &cpu{fz: fz, fn: fn, fh: fh}
		y := sys.dec(x)
		return y == x-1 &&
			sys.fz == (y == 0) &&
			sys.fn &&
			sys.fh == (x&0x0F == 0)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestADD(t *testing.T) {
	f := func(a, x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{a: a, fz: fz, fn: fn, fh: fh, fc: fc}
		sys.add(x)
		y := a + x
		return sys.a == y &&
			sys.fz == (y == 0) &&
			!sys.fn &&
			sys.fh == (y&0x0F < a&0x0F) &&
			sys.fc == (y < a)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestADC(t *testing.T) {
	f := func(a, x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{a: a, fz: fz, fn: fn, fh: fh, fc: fc}
		sys.adc(x)
		c := 0
		if fc {
			c = 1
		}
		y := int(a) + int(x) + c
		return sys.a == byte(y) &&
			sys.fz == (byte(y) == 0) &&
			!sys.fn &&
			sys.fh == (a&0x0F+x&0x0F+byte(c) > 0x0F) &&
			sys.fc == (y > 0xFF)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestSUB(t *testing.T) {
	f := func(a, x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{a: a, fz: fz, fn: fn, fh: fh, fc: fc}
		sys.sub(x)
		y := int(a) - int(x)
		return sys.a == byte(y) &&
			sys.fz == (byte(y) == 0) &&
			sys.fn &&
			sys.fh == (byte(y&0x0F) > a&0x0F) &&
			sys.fc == (y < 0)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestSBC(t *testing.T) {
	f := func(a, x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{a: a, fz: fz, fn: fn, fh: fh, fc: fc}
		sys.sbc(x)
		c := 0
		if fc {
			c = 1
		}
		y := int(a) - int(x) - c
		return sys.a == byte(y) &&
			sys.fz == (byte(y) == 0) &&
			sys.fn &&
			sys.fh == (a&0x0F < x&0x0F+byte(c)) &&
			sys.fc == (y < 0)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestCP(t *testing.T) {
	f := func(a, x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{a: a, fz: fz, fn: fn, fh: fh, fc: fc}
		sys.cp(x)
		y := int(a) - int(x)
		return sys.fz == (byte(y) == 0) &&
			sys.fn &&
			sys.fh == (byte(y&0x0F) > a&0x0F) &&
			sys.fc == (y < 0)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestRLC(t *testing.T) {
	f := func(x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{fz: fz, fn: fn, fh: fh, fc: fc}
		y := sys.rlc(x)
		return y == (x<<1|x>>7) &&
			sys.fz == (y == 0) &&
			sys.fn == false &&
			sys.fh == false &&
			sys.fc == (x&0x80 == 0x80)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestRRC(t *testing.T) {
	f := func(x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{fz: fz, fn: fn, fh: fh, fc: fc}
		y := sys.rrc(x)
		return y == (x>>1|x<<7) &&
			sys.fz == (y == 0) &&
			sys.fn == false &&
			sys.fh == false &&
			sys.fc == (x&0x01 == 0x01)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestRL(t *testing.T) {
	f := func(x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{fz: fz, fn: fn, fh: fh, fc: fc}
		y := sys.rl(x)
		c := byte(0)
		if fc {
			c = 1
		}
		return y == (x<<1|c) &&
			sys.fz == (y == 0) &&
			sys.fn == false &&
			sys.fh == false &&
			sys.fc == (x&0x80 == 0x80)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestRR(t *testing.T) {
	f := func(x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{fz: fz, fn: fn, fh: fh, fc: fc}
		y := sys.rr(x)
		c := byte(0)
		if fc {
			c = 0x80
		}
		return y == (x>>1|c) &&
			sys.fz == (y == 0) &&
			sys.fn == false &&
			sys.fh == false &&
			sys.fc == (x&0x01 == 0x01)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestSLA(t *testing.T) {
	f := func(x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{fz: fz, fn: fn, fh: fh, fc: fc}
		y := sys.sla(x)
		return y == x<<1 &&
			sys.fz == (y == 0) &&
			sys.fn == false &&
			sys.fh == false &&
			sys.fc == (x&0x80 == 0x80)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestSRA(t *testing.T) {
	f := func(x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{fz: fz, fn: fn, fh: fh, fc: fc}
		y := sys.sra(x)
		return y == (x&0x80|x>>1) &&
			sys.fz == (y == 0) &&
			sys.fn == false &&
			sys.fh == false &&
			sys.fc == (x&0x01 == 0x01)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestSRL(t *testing.T) {
	f := func(x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{fz: fz, fn: fn, fh: fh, fc: fc}
		y := sys.srl(x)
		return y == x>>1 &&
			sys.fz == (y == 0) &&
			sys.fn == false &&
			sys.fh == false &&
			sys.fc == (x&0x01 == 0x01)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestSWAP(t *testing.T) {
	f := func(x byte, fz, fn, fh, fc bool) bool {
		sys := &cpu{fz: fz, fn: fn, fh: fh, fc: fc}
		y := sys.swap(x)
		return y == (x>>4|x<<4) &&
			sys.fz == (y == 0) &&
			sys.fn == false &&
			sys.fh == false &&
			sys.fc == false
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
