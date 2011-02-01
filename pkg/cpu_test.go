package gameboy

import (
	"testing"
	"testing/quick"
)

type testmem struct {
	data []byte
}

func (m *testmem) ReadByte(addr uint16) byte {
	return 0
}

func (m *testmem) WriteByte(addr uint16, x byte) {
}

func (m *testmem) ReadWord(addr uint16) uint16 {
	return 0
}

func (m *testmem) WriteWord(addr uint16, x uint16) {
}

func (m *testmem) ReadPort(addr uint16) byte {
	return 0
}

func (m *testmem) WritePort(addr uint16, x byte) {
}

func (m *testmem) UpdateTimers(t int) {}

func (cpu *CPU) prepareTest(implicit int) {
}

func (cpu *CPU) checkResult(t int) bool {
	return true
}

func TestFetchDecodeExecute(t *testing.T) {
	f := func(data []byte, implicit int) (result bool) {
		mem := &testmem{data:data}
		defer func() {
			if err := recover(); err != nil {
				switch data[0] {
				case 0xD3: fallthrough
				case 0xDB: fallthrough
				case 0xDD: fallthrough
				case 0xE3: fallthrough
				case 0xE4: fallthrough
				case 0xEB: fallthrough
				case 0xEC: fallthrough
				case 0xED: fallthrough
				case 0xF4: fallthrough
				case 0xFC: fallthrough
				case 0xFD: result = true
				default: t.Error(err)
				}
			}
		}()
		cpu := &CPU{mmu: mem}
		cpu.prepareTest(implicit)
		t := cpu.fdx()
		return cpu.checkResult(t)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
