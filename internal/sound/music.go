// Package sound replays the original tracker modules through go-zikmu.
package sound

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	zikmu "github.com/olivierh59500/go-zikmu"
	"github.com/olivierh59500/go-zikmu/ebitenaudio"

	"krytonegg/assets"
	"krytonegg/internal/game"
)

const sampleRate = 44100

// Audio owns one Ebitengine context and two original ProTracker songs.
type Audio struct {
	context *audio.Context
	music   map[string]*ebitenaudio.Player
	current string
	muted   bool
	paused  bool
	samples map[string][]byte
	voices  []*audio.Player
}

// New loads the disk's music with a pure Go decoder and mixer.
func New(muted bool) (*Audio, error) {
	a := &Audio{
		context: audio.NewContext(sampleRate),
		music:   make(map[string]*ebitenaudio.Player),
		samples: make(map[string][]byte),
		muted:   muted,
	}
	for _, name := range []string{"intro", "halloffame"} {
		data, err := assets.Read("original/" + name)
		if err != nil {
			return nil, err
		}
		module, err := zikmu.Load(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("load original %s module: %w", name, err)
		}
		cfg := zikmu.DefaultConfig()
		cfg.SampleRate = sampleRate
		cfg.Channels = 2
		source, err := zikmu.NewPlayer(module, cfg)
		if err != nil {
			return nil, err
		}
		player, err := ebitenaudio.NewPlayer(a.context, source, ebitenaudio.Options{BufferSize: 80 * time.Millisecond})
		if err != nil {
			return nil, err
		}
		if err := player.SetVolume(0.55); err != nil {
			return nil, err
		}
		a.music[name] = player
	}
	names, err := assets.Names()
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if !strings.HasPrefix(name, "audio/") || !strings.HasSuffix(name, ".wav") {
			continue
		}
		data, err := assets.Read(name)
		if err != nil {
			return nil, err
		}
		decoded, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decode original sample %s: %w", name, err)
		}
		pcm, err := io.ReadAll(decoded)
		if err != nil {
			return nil, err
		}
		key := strings.TrimSuffix(strings.TrimPrefix(name, "audio/"), ".wav")
		a.samples[key] = pcm
	}
	return a, nil
}

// Event plays the disk sample associated with a simulation event.
func (a *Audio) Event(event game.Event) {
	name := ""
	switch event.Kind {
	case game.Bounce:
		if event.Value == 1 {
			name = "paddle"
		} else if event.Value == 2 {
			name = "metal"
		}
	case game.BrickHit:
		name = "brick"
	case game.BrickBreak:
		if event.Effect != game.NoEffect {
			name = "bonus-brick"
		}
	case game.BonusCaught:
		name = "collect"
		if event.Effect == game.ExtraLife {
			name = "extra-life"
		}
		if event.Effect == game.Grow {
			name = "grow"
		}
	case game.LevelStarted:
		name = "start"
	case game.LifeLost:
		if event.Value == 0 {
			name = "game-over"
		}
	case game.EnemyHit:
		name = "enemy-hit"
	case game.CombatStarted:
		name = "start"
	case game.CombatHit:
		name = "enemy-hit"
	case game.CombatCompleted:
		name = "combat-end"
	}
	a.play(name)
}

func (a *Audio) play(name string) {
	if a.muted || a.paused || len(a.samples[name]) == 0 {
		return
	}
	// Dispose completed voices rather than accumulating an audio player per hit.
	active := a.voices[:0]
	for _, voice := range a.voices {
		if voice.IsPlaying() {
			active = append(active, voice)
		} else {
			_ = voice.Close()
		}
	}
	a.voices = active
	if len(a.voices) >= 8 {
		_ = a.voices[0].Close()
		a.voices = a.voices[1:]
	}
	voice := a.context.NewPlayerFromBytes(a.samples[name])
	voice.SetVolume(0.65)
	voice.Play()
	a.voices = append(a.voices, voice)
}

// Music changes songs only at screen transitions, retaining playback on mute.
func (a *Audio) Music(name string) {
	if a.current == name {
		return
	}
	if player := a.music[a.current]; player != nil {
		player.Pause()
	}
	a.current = name
	if player := a.music[name]; player != nil {
		if err := player.Reset(); err == nil && !a.muted && !a.paused {
			player.Play()
		}
	}
}

// SetPaused stops both music and effect voices while the simulation is paused.
func (a *Audio) SetPaused(paused bool) {
	if a.paused == paused {
		return
	}
	a.paused = paused
	if player := a.music[a.current]; player != nil {
		if paused || a.muted {
			player.Pause()
		} else {
			player.Play()
		}
	}
	for _, voice := range a.voices {
		if paused {
			voice.Pause()
		} else if !a.muted {
			voice.Play()
		}
	}
}

// ToggleMute preserves the tracker position when sound is restored.
func (a *Audio) ToggleMute() {
	a.muted = !a.muted
	if player := a.music[a.current]; player != nil {
		if a.muted || a.paused {
			player.Pause()
		} else {
			player.Play()
		}
	}
	if a.muted {
		for _, voice := range a.voices {
			voice.Pause()
		}
	}
}

// Muted reports the current sound preference for the on-screen controls.
func (a *Audio) Muted() bool { return a.muted }

// Close releases the audio backend before the game exits.
func (a *Audio) Close() {
	for _, player := range a.music {
		_ = player.Close()
	}
	for _, voice := range a.voices {
		_ = voice.Close()
	}
}
