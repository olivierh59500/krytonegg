// Package recording captures native game frames and renders original audio
// offline. It never opens a speaker, microphone, desktop, or camera device.
package recording

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"path"
	"strings"

	zikmu "github.com/olivierh59500/go-zikmu"

	"krytonegg/assets"
	"krytonegg/internal/game"
	"krytonegg/internal/paula"
)

const SampleRate = paula.SampleRate

type sample struct {
	values []float32
	rate   int
}

type voice struct {
	source         sample
	position, step float64
}

// Mixer renders stereo PCM using original tracker modules and disk samples.
type Mixer struct {
	music        map[string]zikmu.Player
	samples      map[string]sample
	current      string
	voices       [4]*voice
	nextChannel  int
	buffer       []float32
	pcm          []byte
	selectSample func(game.Event) (string, float64)
}

// NewMixer prepares pure Go decoding without starting a live audio context.
func NewMixer() (*Mixer, error) {
	m := &Mixer{music: make(map[string]zikmu.Player), samples: make(map[string]sample), nextChannel: 1}
	selector, err := paula.NewSampleSelector()
	if err != nil {
		return nil, err
	}
	m.selectSample = selector.Select
	for _, name := range []string{"intro", "halloffame"} {
		data, err := assets.Read("original/" + name)
		if err != nil {
			return nil, err
		}
		module, err := zikmu.Load(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("decode original music %s: %w", name, err)
		}
		cfg := zikmu.DefaultConfig()
		cfg.SampleRate, cfg.Channels = SampleRate, 2
		player, err := zikmu.NewPlayer(module, cfg)
		if err != nil {
			return nil, err
		}
		if err := player.SetVolume(0.55); err != nil {
			return nil, err
		}
		m.music[name] = player
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
		source, err := decodeWAV(data)
		if err != nil {
			return nil, fmt.Errorf("decode original sample %s: %w", name, err)
		}
		m.samples[strings.TrimSuffix(path.Base(name), ".wav")] = source
	}
	return m, nil
}

// SetSampleSelector shares event selection and playback-rate changes with the
// runtime sound layer without importing its live audio backend.
func (m *Mixer) SetSampleSelector(selector func(game.Event) (string, float64)) {
	if selector != nil {
		m.selectSample = selector
	}
}

// Render advances exactly the requested stereo frame count. Paused frames are
// silent and retain tracker and sample positions. Returned memory is reused.
func (m *Mixer) Render(frames int, events []game.Event, music string, paused bool) ([]byte, error) {
	if frames < 0 {
		return nil, fmt.Errorf("negative audio frame count")
	}
	if music != m.current {
		m.current = music
		if player := m.music[music]; player != nil {
			if err := player.Reset(); err != nil {
				return nil, err
			}
		}
	}
	count := frames * 2
	if cap(m.buffer) < count {
		m.buffer = make([]float32, count)
	} else {
		m.buffer = m.buffer[:count]
		clear(m.buffer)
	}
	if cap(m.pcm) < count*2 {
		m.pcm = make([]byte, count*2)
	} else {
		m.pcm = m.pcm[:count*2]
		clear(m.pcm)
	}
	if paused {
		return m.pcm, nil
	}
	if player := m.music[m.current]; player != nil {
		if _, err := player.Render(m.buffer); err != nil {
			return nil, err
		}
	}
	// The original Paula queue pops the latest request first. Playback is
	// scheduled per native update, not per emulated Copper interrupt.
	for _, event := range paula.OrderedEvents(events) {
		name, rate := m.selectSample(event)
		source := m.samples[name]
		if len(source.values) == 0 {
			continue
		}
		if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			rate = 1
		}
		channel := m.nextChannel
		m.nextChannel = paula.NextPaulaChannel(channel)
		// A new DMA sample replaces the previous voice on this channel.
		m.voices[channel] = &voice{source: source, step: float64(source.rate) * rate / SampleRate}
	}
	gain := float32(paula.EffectVolume(m.music[m.current] != nil))
	for channel, v := range m.voices {
		if v == nil {
			continue
		}
		side := paula.PaulaStereoSide(channel)
		for frame := 0; frame < frames && v.position < float64(len(v.source.values)); frame++ {
			position := int(v.position)
			value := v.source.values[position]
			if position+1 < len(v.source.values) {
				fraction := float32(v.position - float64(position))
				value += (v.source.values[position+1] - value) * fraction
			}
			m.buffer[frame*2+side] += value * gain
			v.position += v.step
		}
		if v.position >= float64(len(v.source.values)) {
			m.voices[channel] = nil
		}
	}
	for i, value := range m.buffer {
		pcm := int(math.Round(float64(value) * 32768))
		pcm = max(-32768, min(32767, pcm))
		binary.LittleEndian.PutUint16(m.pcm[i*2:], uint16(int16(pcm)))
	}
	return m.pcm, nil
}

func decodeWAV(data []byte) (sample, error) {
	var source sample
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return source, fmt.Errorf("invalid RIFF WAV")
	}
	channels, bits, format := 0, 0, 0
	var payload []byte
	for offset := 12; offset+8 <= len(data); {
		length := int(binary.LittleEndian.Uint32(data[offset+4:]))
		start, end := offset+8, offset+8+length
		if end < start || end > len(data) {
			return source, fmt.Errorf("truncated WAV chunk")
		}
		switch string(data[offset : offset+4]) {
		case "fmt ":
			if length < 16 {
				return source, fmt.Errorf("truncated WAV format")
			}
			format = int(binary.LittleEndian.Uint16(data[start:]))
			channels = int(binary.LittleEndian.Uint16(data[start+2:]))
			source.rate = int(binary.LittleEndian.Uint32(data[start+4:]))
			bits = int(binary.LittleEndian.Uint16(data[start+14:]))
		case "data":
			payload = data[start:end]
		}
		offset = end + (length & 1)
	}
	if format != 1 || (channels != 1 && channels != 2) || (bits != 8 && bits != 16) || source.rate <= 0 || len(payload) == 0 {
		return source, fmt.Errorf("unsupported or empty original PCM WAV")
	}
	stride := channels * bits / 8
	if len(payload)%stride != 0 {
		return source, fmt.Errorf("unaligned PCM WAV")
	}
	source.values = make([]float32, len(payload)/stride)
	for frame := range source.values {
		for channel := 0; channel < channels; channel++ {
			position := frame*stride + channel*bits/8
			if bits == 8 {
				source.values[frame] += float32(int(payload[position])-128) / 128
			} else {
				source.values[frame] += float32(int16(binary.LittleEndian.Uint16(payload[position:]))) / 32768
			}
		}
		source.values[frame] /= float32(channels)
	}
	return source, nil
}
