// Copyright 2011 Kevin Bulusek. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gameboy

import (
	"fmt"
	"testing"
	"testing/quick"
)

// Convert a number to its BCD-encoded representation.
func tobcd(x byte) byte {
	return ((x/10)%10)<<4 | x%10
}

// Add two BCD numbers.
func addbcd(x, y byte) (z byte, fh, fc bool) {
	ones := x&0x0F + y&0x0F
	tens := x>>4 + y>>4
	if ones >= 10 {
		tens++
		ones -= 10
		fh = true
	}
	if tens >= 10 {
		tens -= 10
		fc = true
	}
	return tens<<4 | ones, fh, fc
}

// Subract two BCD numbers.
func subbcd(x, y byte) (z byte, fh, fc bool) {
	ones := int(x&0x0F) - int(y&0x0F)
	tens := int(x>>4) - int(y>>4)
	if ones < 0 {
		tens--
		ones += 10
		fh = true
	}
	if tens < 0 {
		tens += 10
		fc = true
	}
	return byte(tens<<4 | ones), fh, fc
}

func TestToBCD(t *testing.T) {
	f := func(x byte) bool {
		y := tobcd(x)
		return fmt.Sprintf("%d", x%100) == fmt.Sprintf("%x", y)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestAddBCD(t *testing.T) {
	f := func(x, y byte) bool {
		a := tobcd(x)
		b := tobcd(y)
		c, fh, fc := addbcd(a, b)

		x %= 100
		y %= 100
		z := (x + y)
		fh = (fh && z%10 < x%10) || (!fh && z%10 >= x%10)
		fc = (fc && z >= 100) || (!fc && z < 100)
		z %= 100

		return fmt.Sprintf("%d+%d=%d", x, y, z) ==
			fmt.Sprintf("%x+%x=%x", a, b, c) &&
			fh && fc
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestSubBCD(t *testing.T) {
	f := func(x, y byte) bool {
		a := tobcd(x)
		b := tobcd(y)
		c, fh, fc := subbcd(a, b)

		x %= 100
		y %= 100
		z := int(x) - int(y)
		if z < 0 {
			z += 100
		} else {
			fc = !fc
		}
		fh = (fh && byte(z%10) > x%10) || (!fh && byte(z%10) <= x)

		return fmt.Sprintf("%d-%d=%d", x, y, z) ==
			fmt.Sprintf("%x-%x=%x", a, b, c) &&
			fh && fc
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestDAA(t *testing.T) {
	f := func(a, b byte) bool {
		a = tobcd(a)
		b = tobcd(b)
		result, fh, fc := addbcd(a, b)

		sys := &cpu{a: a}
		sys.add(b)
		sys.daa()

		return sys.a == result &&
			sys.fh == fh &&
			sys.fc == fc
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestDAS(t *testing.T) {
	f := func(a, b byte) bool {
		a = tobcd(a)
		b = tobcd(b)
		result, fh, fc := subbcd(a, b)

		sys := &cpu{a: a}
		sys.sub(b)
		sys.das()

		return sys.a == result &&
			sys.fh == fh &&
			sys.fc == fc
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
