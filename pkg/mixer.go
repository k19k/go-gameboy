// Copyright 2011 Kevin Bulusek. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gameboy

import (
	"fmt"
	"rand"
	"⚛sdl"
	"⚛sdl/audio"
)

const (
	ticksFreq      = 1 << 20
	mixerStepTicks = 4096 // 1/256 second
	mvolmax        = 24000
	chvolmax       = 6000
)

// Lookup tables for volume levels.  Each step is calculated using:
//   log(i+1) * max
// where the log base is the number of steps, i is the index and max
// is the relevant maximum.
var volstep []int16
var mvolstep []int16

type sound struct {
	length     int
	volumeInit int
	volumeDir  int
	volumeTime int
	loop       bool
	init       bool

	clock  int // incremented at 256Hz
	volume int
	active bool
	phase  int
}

func (ch *sound) step() {
	switch {
	case ch.init:
		ch.init = false
		ch.active = true
		ch.volume = ch.volumeInit
		ch.clock = 0
		ch.phase = 0
	case ch.clock < (64 - ch.length):
		ch.clock++
	case ch.loop:
		ch.clock = 0
	default:
		ch.active = false
	}

	if ch.volumeTime != 0 && ch.clock > 0 {
		if (ch.clock % (4 * ch.volumeTime)) == 0 {
			ch.volume += ch.volumeDir
			if ch.volume > 0x0F {
				ch.volume = 0x0F
			} else if ch.volume < 0 {
				ch.volume = 0
			}
		}
	}
}

type tone struct {
	sound

	waveDuty int
	freq     int

	period int
	duty   int
}

func (ch *tone) step(afreq int) {
	ch.sound.step()

	ch.calcPeriod(afreq)
}

func (ch *tone) calcPeriod(afreq int) {
	if ch.freq == 0 {
		ch.active = false
		return
	} else if ch.freq > afreq/2 {
		ch.freq = afreq / 2
	}

	// TODO adjust for remainder
	ch.period = afreq / ch.freq

	switch ch.waveDuty {
	case 0:
		ch.duty = ch.period / 8
	case 1:
		ch.duty = ch.period / 4
	case 2:
		ch.duty = ch.period / 2
	case 3:
		ch.duty = ch.period * 3 / 4
	}
}

func (ch *tone) mix(buf []int16, onleft, onright int16) {
	if ch.volume == 0 {
		return
	}
	amp := volstep[ch.volume]
	for f := 0; f < len(buf); f += 2 {
		if ch.phase >= ch.period {
			ch.phase = 0
		}
		if ch.phase < ch.duty {
			buf[f] += amp * onleft
			buf[f+1] += amp * onright
		} else {
			buf[f] += -amp * onleft
			buf[f+1] += -amp * onright
		}
		ch.phase++
	}
}

type tonesweep struct {
	tone

	sweepTime  int
	sweepDir   int
	sweepShift uint
}

func (ch *tonesweep) step(afreq int) {
	ch.tone.step(afreq)

	if ch.sweepTime != 0 && ch.clock > 0 {
		if (ch.clock % (2 * ch.sweepTime)) == 0 {
			ch.freq += ch.sweepDir * ch.freq / (1 << ch.sweepShift)
			ch.calcPeriod(afreq)
		}
	}
}

type noise struct {
	sound

	shiftClockFreq   uint
	counterStepWidth int
	dividingRatio    int

	afreq  int
	freq   int
	period int
	sign   int16
}

func (ch *noise) step() {
	ch.sound.step()

	freq := 524288
	if ch.dividingRatio == 0 {
		freq *= 2
	} else {
		freq /= ch.dividingRatio
	}
	freq /= 1 << ch.shiftClockFreq

	ch.freq = freq
}

func (ch *noise) calcPeriod() {
	// no, I am not sure this is right. Thank you for asking.
	ch.period = int(rand.Int31()+1) & ch.counterStepWidth
	ch.period = ch.period * ch.afreq / ch.freq
}

func (ch *noise) mix(buf []int16, onleft, onright int16) {
	if ch.volume == 0 {
		return
	}
	amp := volstep[ch.volume]
	for f := 0; f < len(buf); f += 2 {
		if ch.phase >= ch.period {
			ch.phase = 0
			ch.sign = -ch.sign
			ch.calcPeriod()
		}
		buf[f] += amp * ch.sign * onleft
		buf[f+1] += amp * ch.sign * onright
		ch.phase++
	}
}

type wave struct {
	on     bool
	length int
	level  int
	freq   int
	init   bool
	loop   bool

	clock  int
	active bool
	period int
	phase  int
}

func (ch *wave) step(afreq int) {
	switch {
	case !ch.on:
		ch.active = false
	case ch.init:
		ch.init = false
		ch.active = true
		ch.clock = 0
		ch.phase = 0
	case ch.clock < (256 - ch.length):
		ch.clock++
	case ch.loop:
		ch.clock = 0
	default:
		ch.active = false
	}

	ch.period = afreq / ch.freq
}

func (ch *wave) mix(buf []int16, onleft, onright int16, mem *memory) {
	if ch.level == 0 {
		return
	}
	for f := 0; f < len(buf); f += 2 {
		if ch.phase >= ch.period {
			ch.phase = 0
		}
		// TODO verify this
		pos := uint16(ch.phase * 32 / ch.period)
		sample := int(mem.readPort(portWAVE + pos>>1))
		sample >>= (1 - pos&1) << 2
		sample &= 0x0F
		if sample >= 8 {
			sample -= 15
		}
		sample >>= uint(ch.level) - 1
		var amp int16
		if sample > 0 {
			amp = volstep[sample*2]
		} else {
			amp = -volstep[-sample*2]
		}
		buf[f] += amp * onleft
		buf[f+1] += amp * onright
		ch.phase++
	}
}

type mixer struct {
	audio.AudioSpec
	*memory

	clock int

	buf   [][]int16
	bufi  int
	frame uint

	send   chan []int16
	status chan bool
	quit   chan int

	enable bool

	volL int16
	volR int16

	ch1L, ch1R int16
	ch2L, ch2R int16
	ch3L, ch3R int16
	ch4L, ch4R int16

	ch1 tonesweep
	ch2 tone
	ch3 wave
	ch4 noise
}

func NewMixer(mem *memory) (mix *mixer, err interface{}) {
	spec := audio.AudioSpec{
		Freq:     mem.config.AudioFreq,
		Format:   audio.AUDIO_S16SYS,
		Channels: 2,
		Samples:  2048}

	if audio.OpenAudio(&spec, &spec) != 0 {
		return nil, sdl.GetError()
	}

	if mem.config.AudioBuffers < 3 {
		mem.config.AudioBuffers = 3
	}

	if mem.config.Verbose {
		fmt.Println("Opened audio:")
		fmt.Printf("  rate:        %dHz\n", spec.Freq)
		fmt.Printf("  channels:    %d\n", spec.Channels)
		fmt.Printf("  buffer size: %d samples\n", spec.Samples)
		fmt.Printf("  buffers:     %d\n", mem.config.AudioBuffers)
	}

	mix = &mixer{AudioSpec: spec, memory: mem}

	mix.buf = make([][]int16, mem.config.AudioBuffers)
	for i := 0; i < len(mix.buf); i++ {
		mix.buf[i] = make([]int16, mix.Samples*2)
	}

	mix.ch4.afreq = spec.Freq
	mix.ch4.sign = 1

	mix.send = make(chan []int16, len(mix.buf)-2)
	mix.status = make(chan bool)
	mix.quit = make(chan int)

	go runAudio(mix.send, mix.status, mix.quit)

	return mix, nil
}

func (mix *mixer) close() {
	mix.quit <- 1
	<-mix.quit // wait for audio thread to finish
}

func (mix *mixer) pause(on bool) {
	mix.status <- on
	mix.enable = !on
}

func (mix *mixer) step(t int) {
	mix.clock += t
	if mix.clock >= mixerStepTicks {
		mix.clock -= mixerStepTicks
		mix.ch1.step(mix.Freq)
		mix.ch2.step(mix.Freq)
		mix.ch3.step(mix.Freq)
		mix.ch4.step()
		mix.mix()
	}
}

func (mix *mixer) mix() {
	frames := 2 * (mixerStepTicks * uint(mix.Freq) / ticksFreq)
	size := uint(len(mix.buf[mix.bufi]))
	if mix.frame+frames >= size {
		slice := size - mix.frame
		mix.slice(slice)
		mix.next()
		frames -= slice
	}
	for frames >= size {
		mix.slice(size)
		mix.next()
		frames -= size
	}
	mix.slice(frames)
}

func (mix *mixer) slice(frames uint) {
	buf := mix.buf[mix.bufi][mix.frame : mix.frame+frames]

	for f := 0; f < len(buf); f++ {
		buf[f] = 0
	}

	if mix.ch1.active {
		mix.ch1.mix(buf, mix.ch1L, mix.ch1R)
	}
	if mix.ch2.active {
		mix.ch2.mix(buf, mix.ch2L, mix.ch2R)
	}
	if mix.ch3.active {
		mix.ch3.mix(buf, mix.ch3L, mix.ch3R, mix.memory)
	}
	if mix.ch4.active {
		mix.ch4.mix(buf, mix.ch4L, mix.ch4R)
	}

	for f := 0; f < len(buf); f += 2 {
		buf[f] = int16(int(buf[f]) * int(mvolstep[mix.volL]) / mvolmax)
		buf[f+1] = int16(int(buf[f+1]) * int(mvolstep[mix.volR]) / mvolmax)
	}

	mix.frame += frames
}

func (mix *mixer) next() {
	mix.send <- mix.buf[mix.bufi]
	mix.bufi++
	if mix.bufi >= len(mix.buf) {
		mix.bufi = 0
	}
	mix.frame = 0
}

func runAudio(send <-chan []int16, status <-chan bool, quit chan int) {
	for {
		select {
		case buf := <-send:
			audio.SendAudio_int16(buf)
		case pause := <-status:
			audio.PauseAudio(pause)
		case <-quit:
			audio.CloseAudio()
			quit <- 1
			return
		}
	}
}

func init() {
	// Pre-calculated tables. Numbers are rounded (not truncated).
	volstep = []int16{
		0, 1500, 2377, 3000, 3483, 3877, 4211, 4500,
		4755, 4983, 5189, 5377, 5551, 5711, 5860, 6000,
	}
	mvolstep = []int16{
		0, 8000, 12680, 16000, 18575, 20680, 22459, 24000,
	}
}
