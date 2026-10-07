package recording

import (
	"bytes"
	"encoding/binary"
	"testing"

	"krytonegg/internal/game"
)

func TestOfflineMusicAndPauseKeepDeterministicPosition(t *testing.T) {
	mixer, err := NewMixer()
	if err != nil {
		t.Fatal(err)
	}
	first, err := mixer.Render(4410, nil, "intro", false)
	if err != nil {
		t.Fatal(err)
	}
	first = append([]byte(nil), first...)
	if bytes.Equal(first, make([]byte, len(first))) {
		t.Fatal("original tracker rendered only silence")
	}
	position := mixer.music["intro"].Position()
	paused, err := mixer.Render(882, nil, "intro", true)
	if err != nil || !bytes.Equal(paused, make([]byte, len(paused))) || mixer.music["intro"].Position() != position {
		t.Fatal("pause advanced the tracker", err)
	}
	second, err := mixer.Render(4410, nil, "", false)
	if err != nil || !bytes.Equal(second, make([]byte, len(second))) {
		t.Fatal("stopped music was not silent", err)
	}
	restarted, err := mixer.Render(4410, nil, "intro", false)
	if err != nil || !bytes.Equal(restarted, first) {
		t.Fatal("music transition did not reset the original module", err)
	}
}

func TestPausedEffectVoiceResumesWithoutSkippingSamples(t *testing.T) {
	mixer := &Mixer{nextChannel: 1, samples: map[string]sample{"test": {values: []float32{0.25, 0.5, 0.75}, rate: SampleRate}}, music: nil, selectSample: func(game.Event) (string, float64) { return "test", 1 }}
	first, err := mixer.Render(1, []game.Event{{Kind: game.BrickHit}}, "", false)
	if err != nil || int16(binary.LittleEndian.Uint16(first[2:])) != 3686 || binary.LittleEndian.Uint16(first) != 0 {
		t.Fatal(first, err)
	}
	paused, err := mixer.Render(20, nil, "", true)
	if err != nil || !bytes.Equal(paused, make([]byte, len(paused))) {
		t.Fatal(err)
	}
	resumed, err := mixer.Render(1, nil, "", false)
	if err != nil || int16(binary.LittleEndian.Uint16(resumed[2:])) != 7373 || binary.LittleEndian.Uint16(resumed) != 0 {
		t.Fatal(resumed, err)
	}
}

func TestDecodeRejectsTruncatedOriginalWave(t *testing.T) {
	if _, err := decodeWAV([]byte("RIFF0000WAVEdata")); err == nil {
		t.Fatal("truncated WAV was accepted")
	}
}

func TestFourPaulaVoicesReplaceInOriginalOrderAndKeepStereo(t *testing.T) {
	mixer := &Mixer{
		nextChannel: 1,
		samples: map[string]sample{
			"a": {values: []float32{0.125, 0.125}, rate: SampleRate},
			"b": {values: []float32{0.25, 0.25}, rate: SampleRate},
			"c": {values: []float32{0.375, 0.375}, rate: SampleRate},
			"d": {values: []float32{0.5, 0.5}, rate: SampleRate},
			"e": {values: []float32{0.625, 0.625}, rate: SampleRate},
		},
		selectSample: func(event game.Event) (string, float64) { return event.Sound, 1 },
	}
	events := []game.Event{{Sound: "a"}, {Sound: "b"}, {Sound: "c"}, {Sound: "d"}, {Sound: "e"}}
	pcm, err := mixer.Render(1, events, "", false)
	if err != nil {
		t.Fatal(err)
	}
	// Newest-first requests use channels 1,0,3,2,1. The fifth request replaces
	// the first, leaving d+c on the left and a+b on the right.
	left, right := int16(binary.LittleEndian.Uint16(pcm)), int16(binary.LittleEndian.Uint16(pcm[2:]))
	if left != 12902 || right != 5530 || mixer.nextChannel != 0 {
		t.Fatalf("four-channel output = (%d,%d), next = %d", left, right, mixer.nextChannel)
	}
	for _, voice := range mixer.voices {
		if voice == nil || voice.position != 1 {
			t.Fatal("one of the four original voices was missing or skipped", voice)
		}
	}
}

func TestRecoveredShotPeriodProducesStereoAudioWithoutADevice(t *testing.T) {
	mixer, err := NewMixer()
	if err != nil {
		t.Fatal(err)
	}
	base, err := NewMixer()
	if err != nil {
		t.Fatal(err)
	}
	// This is an actual original combat-shot sample, not a generated test tone.
	fast, err := mixer.Render(4410, []game.Event{{Kind: game.ShotFired, Sound: "combat-fire", Period: 0x122}}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	normal, err := base.Render(4410, []game.Event{{Kind: game.ShotFired, Sound: "combat-fire", Period: 0x244}}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	var fastEnergy, normalEnergy int64
	for frame := 0; frame < 4410; frame++ {
		if binary.LittleEndian.Uint16(fast[frame*4:]) != 0 || binary.LittleEndian.Uint16(normal[frame*4:]) != 0 {
			t.Fatal("first Paula channel one leaked into the left stereo side")
		}
		f := int64(int16(binary.LittleEndian.Uint16(fast[frame*4+2:])))
		n := int64(int16(binary.LittleEndian.Uint16(normal[frame*4+2:])))
		fastEnergy += f * f
		normalEnergy += n * n
	}
	if fastEnergy == 0 || normalEnergy == 0 || bytes.Equal(fast, normal) {
		t.Fatal("the recovered shot was silent or ignored its dynamic period")
	}
	if mixer.voices[1] == nil || mixer.voices[1].step != base.voices[1].step*2 {
		t.Fatal("halving the source period did not double playback speed")
	}
}

func TestOriginalStereoVoiceHeadroomAvoidsClipping(t *testing.T) {
	mixer := &Mixer{
		nextChannel:  1,
		samples:      map[string]sample{"full": {values: []float32{1, -1}, rate: SampleRate}},
		selectSample: func(game.Event) (string, float64) { return "full", 1 },
	}
	pcm, err := mixer.Render(2, make([]game.Event, 4), "", false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(pcm); i += 2 {
		value := int16(binary.LittleEndian.Uint16(pcm[i:]))
		if value <= -32768 || value >= 32767 || value == 0 {
			t.Fatalf("two original voices on one side clipped or vanished: %d", value)
		}
	}
}
