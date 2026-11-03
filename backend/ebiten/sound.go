package ebiten

import (
	"bytes"
	"io"
	"log"
	"strings"

	"github.com/LaPingvino/gopus"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

// sampleRate is the audio context's rate: Opus' own (the WAVs are resampled
// to it).
const sampleRate = 48000

// sounds plays a project's sounds (runner.Sound): WAVs decoded once, Ogg
// Opus (the music `gameviber import` renders from MIDI) streamed by gopus as
// it plays. A .mid without its .opus is skipped, logged once.
type sounds struct {
	ctx     *audio.Context
	pcm     map[string][]byte
	opus    map[string][]byte
	players map[string]*audio.Player
	midi    map[string]bool
	muted   bool // M: everything silent (players keep going, so unmuting picks the music up)
	warned  bool
}

// DecodeSounds decodes a project's WAV sounds to 16-bit stereo PCM at
// sampleRate, by name; the rest (MIDI …) are listed as skipped.
func DecodeSounds(p *gmfile.Project) (pcm map[string][]byte, skipped []string) {
	pcm = map[string][]byte{}
	for _, s := range p.Sounds {
		if s == nil || s.Data == nil {
			continue
		}
		if strings.EqualFold(s.Ext, ".opus") {
			continue // streamed (opusData)
		}
		if !strings.EqualFold(s.Ext, ".wav") {
			skipped = append(skipped, s.Name+s.Ext)
			continue
		}
		st, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(s.Data))
		if err != nil {
			skipped = append(skipped, s.Name+s.Ext+" ("+err.Error()+")")
			continue
		}
		b, err := io.ReadAll(st)
		if err != nil {
			skipped = append(skipped, s.Name+s.Ext+" ("+err.Error()+")")
			continue
		}
		pcm[s.Name] = b
	}
	return pcm, skipped
}

func newSounds(p *gmfile.Project) *sounds {
	pcm, skipped := DecodeSounds(p)
	s := &sounds{ctx: audio.NewContext(sampleRate), pcm: pcm, opus: opusData(p), players: map[string]*audio.Player{}, midi: map[string]bool{}}
	for _, n := range skipped {
		s.midi[strings.SplitN(n, ".", 2)[0]] = true
	}
	return s
}

func (s *sounds) Play(name string, loop bool) {
	if data, ok := s.opus[name]; ok {
		s.playOpus(name, data, loop)
		return
	}
	pcm, ok := s.pcm[name]
	if !ok {
		if s.midi[name] && !s.warned {
			log.Printf("gameviber: %s is MIDI music — not played yet", name)
			s.warned = true
		}
		return
	}
	if p := s.players[name]; p != nil { // GM 5 restarts a sound already playing
		p.Close()
	}
	var src io.Reader = bytes.NewReader(pcm)
	if loop {
		src = audio.NewInfiniteLoop(bytes.NewReader(pcm), int64(len(pcm)))
	}
	p, err := s.ctx.NewPlayer(src)
	if err != nil {
		return
	}
	s.players[name] = p
	p.SetVolume(s.volume())
	p.Play()
}

func (s *sounds) Stop(name string) {
	if p := s.players[name]; p != nil {
		p.Close()
		delete(s.players, name)
	}
}

func (s *sounds) StopAll() {
	for n := range s.players {
		s.Stop(n)
	}
}

func (s *sounds) Playing(name string) bool {
	p := s.players[name]
	return p != nil && p.IsPlaying()
}

// opusData: the project's Ogg Opus sounds by name.
func opusData(p *gmfile.Project) map[string][]byte {
	m := map[string][]byte{}
	for _, s := range p.Sounds {
		if s != nil && s.Data != nil && strings.EqualFold(s.Ext, ".opus") {
			m[s.Name] = s.Data
		}
	}
	return m
}

// playOpus streams an Ogg Opus sound (looped through the stream's seeking).
func (s *sounds) playOpus(name string, data []byte, loop bool) {
	st, err := gopus.NewStream(bytes.NewReader(data))
	if err != nil {
		log.Printf("gameviber: %s: %v", name, err)
		return
	}
	var src io.Reader = st
	if loop {
		src = audio.NewInfiniteLoop(st, st.Length())
	}
	if p := s.players[name]; p != nil {
		p.Close()
	}
	p, err := s.ctx.NewPlayer(src)
	if err != nil {
		return
	}
	s.players[name] = p
	p.SetVolume(s.volume())
	p.Play()
}

func (s *sounds) volume() float64 {
	if s.muted {
		return 0
	}
	return 1
}

// SetMuted turns all sound off or on (M) — as Stelo's and Pacmanio's button.
func (s *sounds) SetMuted(m bool) {
	s.muted = m
	for _, p := range s.players {
		p.SetVolume(s.volume())
	}
}
