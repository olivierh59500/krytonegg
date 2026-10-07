// Package paula shares recovered sample selection and stereo routing without
// initializing a live audio backend.
package paula

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"

	"krytonegg/assets"
	"krytonegg/internal/game"
)

// SampleRate is shared by live playback and the silent presentation renderer.
const SampleRate = 44100

// SampleSelector resolves recovered sample names and Paula playback periods.
// The original period table comes from the local extraction manifest.
type SampleSelector struct{ periods map[string]int }

// NewSampleSelector loads metadata without opening an audio device.
func NewSampleSelector() (*SampleSelector, error) {
	data, err := assets.Read("manifest.json")
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Sounds map[string]struct {
			Period int `json:"period"`
		} `json:"sounds"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("read original sound periods: %w", err)
	}
	s := &SampleSelector{periods: make(map[string]int, len(manifest.Sounds))}
	for name, item := range manifest.Sounds {
		if item.Period <= 0 {
			return nil, fmt.Errorf("original sample %s has an invalid Paula period", name)
		}
		s.periods[name] = item.Period
	}
	return s, nil
}

// Select gives explicit source events priority over compatibility fallbacks.
// Paula frequency is inversely proportional to its period, so this returns
// the original WAV's period divided by the requested event period.
func (s *SampleSelector) Select(event game.Event) (string, float64) {
	name := event.Sound
	if name == "" {
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
		case game.ReserveEarned:
			name = "extra-life"
		case game.LevelStarted:
			name = "start"
		case game.EnemyHit, game.CombatHit:
			name = "enemy-hit"
		case game.PaddleDestroyed:
			name = "game-over"
			// Ordinary LifeLost follows PaddleDestroyed after the animation.
			// It must not replay the destruction sample a second time. Combat
			// endings identify their source sample explicitly in the event.
		}
	}
	base := s.periods[name]
	if base <= 0 {
		return "", 1
	}
	rate := 1.0
	if event.Period > 0 {
		rate = float64(base) / float64(event.Period)
	}
	return name, rate
}

// OrderedEvents returns the source queue's newest-first update batch. A generic
// damage notification preceding its explicit destruction sound is visual
// bookkeeping, not a second request to play the same brick contact.
func OrderedEvents(events []game.Event) []game.Event {
	result := make([]game.Event, 0, len(events))
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		if event.Kind == game.BrickHit && event.Sound == "" {
			destroyed := false
			for _, candidate := range events[index+1:] {
				if candidate.Kind == game.BrickBreak && candidate.Sound != "" && candidate.X == event.X && candidate.Y == event.Y {
					destroyed = true
					break
				}
			}
			if destroyed {
				continue
			}
		}
		result = append(result, event)
	}
	return result
}

// EffectVolume leaves room for the two original Paula voices on each stereo
// side. Tracker screens reserve additional headroom for a transition effect.
func EffectVolume(withMusic bool) float64 {
	if withMusic {
		return 0.2
	}
	return 0.45
}

// NextPaulaChannel preserves the original decrementing four-channel allocator.
func NextPaulaChannel(channel int) int { return (channel + 3) & 3 }

// PaulaStereoSide preserves the Amiga's fixed channel wiring: zero is left,
// one is right. Spatial coordinates do not pan the original mono samples.
func PaulaStereoSide(channel int) int {
	if channel == 0 || channel == 3 {
		return 0
	}
	return 1
}

// ResampleEffect retimes the decoded original stereo PCM and routes its mono
// sample to the selected Paula side. It creates no new tones or sample data.
func ResampleEffect(pcm []byte, rate float64, channel int) []byte {
	if len(pcm) < 4 || len(pcm)%4 != 0 || rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return nil
	}
	frames := len(pcm) / 4
	outputFrames := int(math.Ceil(float64(frames) / rate))
	result := make([]byte, outputFrames*4)
	mono := func(frame int) float64 {
		left := int16(binary.LittleEndian.Uint16(pcm[frame*4:]))
		right := int16(binary.LittleEndian.Uint16(pcm[frame*4+2:]))
		return (float64(left) + float64(right)) / 2
	}
	side := PaulaStereoSide(channel)
	for frame := 0; frame < outputFrames; frame++ {
		position := float64(frame) * rate
		index := min(int(position), frames-1)
		value := mono(index)
		if index+1 < frames {
			value += (mono(index+1) - value) * (position - float64(index))
		}
		value = math.Max(-32768, math.Min(32767, math.Round(value)))
		binary.LittleEndian.PutUint16(result[frame*4+side*2:], uint16(int16(value)))
	}
	return result
}
