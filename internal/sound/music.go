// Package sound replays the original tracker modules through go-zikmu.
package sound

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	zikmu "github.com/olivierh59500/go-zikmu"
	"github.com/olivierh59500/go-zikmu/ebitenaudio"

	"krytonegg/assets"
	"krytonegg/internal/game"
	"krytonegg/internal/paula"
)

const sampleRate = paula.SampleRate

// Audio owns one Ebitengine context and two original ProTracker songs.
type Audio struct {
	context     *audio.Context
	music       map[string]*ebitenaudio.Player
	current     string
	muted       bool
	paused      bool
	samples     map[string][]byte
	playback    map[effectKey][]byte
	selector    *paula.SampleSelector
	voices      [4]*audio.Player
	nextChannel int
}

type effectKey struct {
	name string
	rate uint64
	side int
}

// New loads the disk's music with a pure Go decoder and mixer.
func New(muted bool) (*Audio, error) {
	selector, err := paula.NewSampleSelector()
	if err != nil {
		return nil, err
	}
	a := &Audio{
		context:     audio.NewContext(sampleRate),
		music:       make(map[string]*ebitenaudio.Player),
		samples:     make(map[string][]byte),
		playback:    make(map[effectKey][]byte),
		muted:       muted,
		selector:    selector,
		nextChannel: 1,
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
	name, rate := a.SelectSample(event)
	a.play(name, rate)
}

// SelectSample shares the exact live selection with the silent movie renderer.
func (a *Audio) SelectSample(event game.Event) (string, float64) {
	if a == nil || a.selector == nil {
		return "", 1
	}
	return a.selector.Select(event)
}

// Events consumes one native update in the original queue's newest-first order.
// Scheduling remains update-based rather than a cycle-exact Copper interrupt.
func (a *Audio) Events(events []game.Event) {
	for _, event := range paula.OrderedEvents(events) {
		a.Event(event)
	}
}

func (a *Audio) play(name string, rate float64) {
	if a.muted || a.paused || len(a.samples[name]) == 0 {
		return
	}
	channel := a.nextChannel
	a.nextChannel = paula.NextPaulaChannel(channel)
	if old := a.voices[channel]; old != nil {
		_ = old.Close()
	}
	key := effectKey{name: name, rate: math.Float64bits(rate), side: paula.PaulaStereoSide(channel)}
	pcm := a.playback[key]
	if pcm == nil {
		pcm = paula.ResampleEffect(a.samples[name], rate, channel)
		// Recovered period changes use a small finite set. Keep practice or
		// future event sources from retaining an unlimited number of variants.
		if len(a.playback) >= 512 {
			clear(a.playback)
		}
		a.playback[key] = pcm
	}
	if len(pcm) == 0 {
		return
	}
	voice := a.context.NewPlayerFromBytes(pcm)
	voice.SetVolume(paula.EffectVolume(a.current != ""))
	voice.Play()
	a.voices[channel] = voice
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
	for _, voice := range a.voices {
		if voice != nil {
			voice.SetVolume(paula.EffectVolume(name != ""))
		}
	}
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
		if voice == nil {
			continue
		}
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
			if voice != nil {
				voice.Pause()
			}
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
		if voice != nil {
			_ = voice.Close()
		}
	}
}
