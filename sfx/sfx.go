// Package sfx generates retro sound effects in the manner of sfxr (DrPetter's
// sound effect generator, 2007): one oscillator — square, sawtooth, sine or
// noise — shaped by an attack / sustain / decay envelope, pitch slides,
// vibrato, an arpeggio jump, a duty sweep, a phaser and low / high-pass
// filters, with presets that pick random settings per kind (a coin, a jump …).
// Written for GameViber from the algorithm's description; output 16-bit mono
// WAV at 44100 Hz.
package sfx

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
)

// Wave shapes.
const (
	Square = iota
	Saw
	Sine
	Noise
)

// Params are the generator's settings, each 0..1 (or -1..1 where it slides),
// as sfxr has them; JSON / TOML friendly.
type Params struct {
	Wave int `json:"wave" toml:"wave"`

	Attack  float64 `json:"attack" toml:"attack"`
	Sustain float64 `json:"sustain" toml:"sustain"`
	Punch   float64 `json:"punch" toml:"punch"`
	Decay   float64 `json:"decay" toml:"decay"`

	Freq       float64 `json:"freq" toml:"freq"`             // start pitch
	FreqLimit  float64 `json:"freq_limit" toml:"freq_limit"` // the slide stops here
	Slide      float64 `json:"slide" toml:"slide"`           // -1..1
	DeltaSlide float64 `json:"delta_slide" toml:"delta_slide"`

	VibDepth float64 `json:"vib_depth" toml:"vib_depth"`
	VibSpeed float64 `json:"vib_speed" toml:"vib_speed"`

	ArpMod   float64 `json:"arp_mod" toml:"arp_mod"` // -1..1: the pitch jump
	ArpSpeed float64 `json:"arp_speed" toml:"arp_speed"`

	Duty      float64 `json:"duty" toml:"duty"` // square's duty cycle
	DutySweep float64 `json:"duty_sweep" toml:"duty_sweep"`

	RepeatSpeed float64 `json:"repeat_speed" toml:"repeat_speed"`

	PhaserOffset float64 `json:"phaser_offset" toml:"phaser_offset"`
	PhaserSweep  float64 `json:"phaser_sweep" toml:"phaser_sweep"`

	LPFCutoff    float64 `json:"lpf_cutoff" toml:"lpf_cutoff"`
	LPFSweep     float64 `json:"lpf_sweep" toml:"lpf_sweep"`
	LPFResonance float64 `json:"lpf_resonance" toml:"lpf_resonance"`
	HPFCutoff    float64 `json:"hpf_cutoff" toml:"hpf_cutoff"`
	HPFSweep     float64 `json:"hpf_sweep" toml:"hpf_sweep"`

	Volume float64 `json:"volume" toml:"volume"`
}

// Default is a plain beep (the generator's neutral settings).
func Default() Params {
	return Params{Wave: Square, Sustain: 0.3, Decay: 0.4, Freq: 0.3, Duty: 0, LPFCutoff: 1, Volume: 0.5}
}

// Presets are the kinds a preset name picks (Preset).
var Presets = []string{"coin", "laser", "explosion", "powerup", "hit", "jump", "blip"}

// Preset is a random sound of a kind, from seed (the same seed, the same sound).
func Preset(kind string, seed uint64) Params {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	f := func(x float64) float64 { return r.Float64() * x }
	p := Default()
	switch kind {
	case "coin": // a pickup: a bright tone, often with a jump up
		p.Wave = Square
		p.Freq = 0.4 + f(0.5)
		p.Sustain, p.Decay, p.Punch = f(0.1), 0.1+f(0.4), 0.3+f(0.3)
		if r.IntN(2) == 0 {
			p.ArpSpeed, p.ArpMod = 0.5+f(0.2), 0.2+f(0.4)
		}
	case "laser": // a shot: a falling pitch
		p.Wave = r.IntN(3)
		if p.Wave == Sine && r.IntN(2) == 0 {
			p.Wave = r.IntN(2)
		}
		p.Freq = 0.5 + f(0.5)
		p.FreqLimit = max(0, p.Freq-0.2-f(0.6))
		p.Slide = -0.15 - f(0.2)
		p.Duty, p.DutySweep = f(0.5), f(0.2)
		p.Sustain, p.Decay, p.Punch = 0.1+f(0.2), f(0.4), f(0.3)
		if r.IntN(3) == 0 {
			p.PhaserOffset, p.PhaserSweep = f(0.2), -f(0.2)
		}
		if r.IntN(2) == 0 {
			p.HPFCutoff = f(0.3)
		}
	case "explosion": // noise falling away
		p.Wave = Noise
		p.Freq = 0.1 + f(0.4)
		p.Slide = -0.1 + f(0.4)
		if r.IntN(5) == 0 {
			p.Slide = 0
		}
		if r.IntN(3) == 0 {
			p.RepeatSpeed = 0.3 + f(0.5)
		}
		p.Sustain, p.Decay, p.Punch = 0.1+f(0.3), f(0.5), 0.2+f(0.6)
		if r.IntN(2) == 0 {
			p.PhaserOffset, p.PhaserSweep = -0.3+f(0.9), -f(0.3)
		}
		if r.IntN(2) == 0 {
			p.VibDepth, p.VibSpeed = f(0.7), f(0.6)
		}
	case "powerup": // a rising tone
		p.Wave = r.IntN(2)
		p.Duty = f(0.6)
		p.Freq = 0.2 + f(0.3)
		p.Slide = 0.1 + f(0.4)
		if r.IntN(2) == 0 {
			p.RepeatSpeed = 0.4 + f(0.4)
		} else if r.IntN(2) == 0 {
			p.VibDepth, p.VibSpeed = f(0.2), 0.4+f(0.4)
		}
		p.Sustain, p.Decay = f(0.4), 0.1+f(0.4)
	case "hit": // a short thud or crack
		p.Wave = r.IntN(3)
		if p.Wave == Sine {
			p.Wave = Noise
		}
		p.Freq = 0.2 + f(0.6)
		p.Slide = -0.3 - f(0.4)
		p.Sustain, p.Decay = f(0.1), 0.1+f(0.2)
		if r.IntN(2) == 0 {
			p.HPFCutoff = f(0.3)
		}
	case "jump": // a short rising square
		p.Wave = Square
		p.Duty = f(0.6)
		p.Freq = 0.3 + f(0.3)
		p.Slide = 0.1 + f(0.2)
		p.Sustain, p.Decay = 0.1+f(0.3), 0.1+f(0.2)
		if r.IntN(2) == 0 {
			p.HPFCutoff = f(0.3)
		}
		if r.IntN(2) == 0 {
			p.LPFCutoff = 1 - f(0.6)
		}
	case "blip": // a menu select
		p.Wave = r.IntN(2)
		if p.Wave == Square {
			p.Duty = f(0.6)
		}
		p.Freq = 0.2 + f(0.4)
		p.Sustain, p.Decay = 0.1+f(0.1), f(0.2)
		p.HPFCutoff = 0.1
	}
	return p
}

// Generate renders the sound as samples in -1..1 at 44100 Hz.
func Generate(p Params) []float32 {
	rng := rand.New(rand.NewPCG(1, 2)) // the noise: the same sound every time
	noise := make([]float64, 32)
	newNoise := func() {
		for i := range noise {
			noise[i] = rng.Float64()*2 - 1
		}
	}
	newNoise()

	var (
		period, maxPeriod, slide, deltaSlide float64
		squareDuty, squareSlide              float64
		arpMod                               float64
		arpLimit, arpTime                    int
	)
	reset := func() { // sfxr's ResetSample(restart): the pitch part, again on each repeat
		period = 100 / (p.Freq*p.Freq + 0.001)
		maxPeriod = 100 / (p.FreqLimit*p.FreqLimit + 0.001)
		slide = 1 - math.Pow(p.Slide, 3)*0.01
		deltaSlide = -math.Pow(p.DeltaSlide, 3) * 0.000001
		squareDuty = 0.5 - p.Duty*0.5
		squareSlide = -p.DutySweep * 0.00005
		if p.ArpMod >= 0 {
			arpMod = 1 - math.Pow(p.ArpMod, 2)*0.9
		} else {
			arpMod = 1 + math.Pow(p.ArpMod, 2)*10
		}
		arpTime = 0
		arpLimit = int(math.Pow(1-p.ArpSpeed, 2)*20000 + 32)
		if p.ArpSpeed == 1 {
			arpLimit = 0
		}
	}
	reset()

	// filters
	fltp, fltdp := 0.0, 0.0
	fltw := math.Pow(p.LPFCutoff, 3) * 0.1
	fltwd := 1 + p.LPFSweep*0.0001
	fltdmp := min(0.8, 5/(1+math.Pow(p.LPFResonance, 2)*20)*(0.01+fltw))
	fltphp := 0.0
	flthp := math.Pow(p.HPFCutoff, 2) * 0.1
	flthpd := 1 + p.HPFSweep*0.0003
	// vibrato
	vibPhase, vibSpeed, vibAmp := 0.0, math.Pow(p.VibSpeed, 2)*0.01, p.VibDepth*0.5
	// envelope
	envLen := [3]int{int(p.Attack * p.Attack * 100000), int(p.Sustain * p.Sustain * 100000), int(p.Decay * p.Decay * 100000)}
	envStage, envTime := 0, 0
	// phaser
	phaserBuf := make([]float64, 1024)
	phaserOff := math.Pow(p.PhaserOffset, 2) * 1020
	if p.PhaserOffset < 0 {
		phaserOff = -phaserOff
	}
	phaserDelta := math.Pow(p.PhaserSweep, 2)
	if p.PhaserSweep < 0 {
		phaserDelta = -phaserDelta
	}
	ipp := 0
	// repeat
	repTime, repLimit := 0, int(math.Pow(1-p.RepeatSpeed, 2)*20000+32)
	if p.RepeatSpeed == 0 {
		repLimit = 0
	}

	phase := 0
	var out []float32
	const supersample = 8
	for len(out) < 44100*10 { // at most ten seconds
		repTime++
		if repLimit != 0 && repTime >= repLimit {
			repTime = 0
			reset()
		}
		arpTime++
		if arpLimit != 0 && arpTime >= arpLimit {
			arpLimit = 0
			period *= arpMod
		}
		slide += deltaSlide
		period *= slide
		if period > maxPeriod {
			period = maxPeriod
			if p.FreqLimit > 0 {
				break // the slide ran into its limit: done
			}
		}
		rperiod := period
		if vibAmp > 0 {
			vibPhase += vibSpeed
			rperiod = period * (1 + math.Sin(vibPhase)*vibAmp)
		}
		iperiod := max(int(rperiod), 8)
		squareDuty = min(max(squareDuty+squareSlide, 0), 0.5)
		// the envelope
		envTime++
		if envTime > envLen[envStage] {
			envTime = 0
			envStage++
			if envStage == 3 {
				break
			}
		}
		var env float64
		switch envStage {
		case 0:
			env = float64(envTime) / float64(max(envLen[0], 1))
		case 1:
			env = 1 + math.Pow(1-float64(envTime)/float64(max(envLen[1], 1)), 1)*2*p.Punch
		case 2:
			env = 1 - float64(envTime)/float64(max(envLen[2], 1))
		}
		// the phaser's offset
		phaserOff += phaserDelta
		iphase := min(int(math.Abs(phaserOff)), 1023)
		if flthpd != 0 {
			flthp = min(max(flthp*flthpd, 0.00001), 0.1)
		}
		sample := 0.0
		for range supersample {
			phase++
			if phase >= iperiod {
				phase %= iperiod
				if p.Wave == Noise {
					newNoise()
				}
			}
			fp := float64(phase) / float64(iperiod)
			var s float64
			switch p.Wave {
			case Square:
				s = 0.5
				if fp >= squareDuty {
					s = -0.5
				}
			case Saw:
				s = 1 - fp*2
			case Sine:
				s = math.Sin(fp * 2 * math.Pi)
			case Noise:
				s = noise[phase*32/iperiod]
			}
			// low-pass (resonant), then high-pass
			pp := fltp
			fltw = min(max(fltw*fltwd, 0), 0.1)
			if p.LPFCutoff != 1 {
				fltdp += (s - fltp) * fltw
				fltdp -= fltdp * fltdmp
			} else {
				fltp, fltdp = s, 0
			}
			fltp += fltdp
			fltphp += fltp - pp
			fltphp -= fltphp * flthp
			s = fltphp
			// the phaser
			phaserBuf[ipp&1023] = s
			s += phaserBuf[(ipp-iphase+1024)&1023]
			ipp = (ipp + 1) & 1023
			sample += s * env
		}
		v := sample / supersample * p.Volume * 2
		out = append(out, float32(min(max(v, -1), 1)))
	}
	return out
}

// WAV is samples as a 16-bit mono WAV file at 44100 Hz.
func WAV(samples []float32) []byte {
	var b bytes.Buffer
	n := len(samples)
	le := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("RIFF")
	le(uint32(36 + n*2))
	b.WriteString("WAVEfmt ")
	le(uint32(16))
	le(uint16(1))
	le(uint16(1))
	le(uint32(44100))
	le(uint32(44100 * 2))
	le(uint16(2))
	le(uint16(16))
	b.WriteString("data")
	le(uint32(n * 2))
	for _, s := range samples {
		le(int16(s * 32767))
	}
	return b.Bytes()
}
