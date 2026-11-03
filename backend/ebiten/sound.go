package ebiten

import (
	"bytes"
	"io"
	"log"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

// sampleRate is the audio context's rate (the WAVs are resampled to it).
const sampleRate = 44100

// sounds plays a project's sounds (runner.Sound): WAV effects and music,
// decoded once. PARTIAL: MIDI (GM 5's .mid music) needs a synthesiser — it
// is skipped, logged once.
type sounds struct {
	ctx     *audio.Context
	pcm     map[string][]byte
	players map[string]*audio.Player
	midi    map[string]bool
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
	s := &sounds{ctx: audio.NewContext(sampleRate), pcm: pcm, players: map[string]*audio.Player{}, midi: map[string]bool{}}
	for _, n := range skipped {
		s.midi[strings.SplitN(n, ".", 2)[0]] = true
	}
	return s
}

func (s *sounds) Play(name string, loop bool) {
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
