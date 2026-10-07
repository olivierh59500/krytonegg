package paula

import (
	"encoding/binary"
	"math"
	"testing"

	"krytonegg/internal/game"
)

func TestRecoveredEventPriorityAndPaulaPeriod(t *testing.T) {
	selector, err := NewSampleSelector()
	if err != nil {
		t.Fatal(err)
	}
	if len(selector.periods) != 21 {
		t.Fatalf("expected the recovered 21 WAV variants, got %d", len(selector.periods))
	}
	for name, period := range selector.periods {
		selected, rate := selector.Select(game.Event{Kind: game.LifeLost, Sound: name, Period: period * 2})
		if selected != name || rate != 0.5 {
			t.Fatalf("explicit %s at doubled period: %s, %g", name, selected, rate)
		}
	}
	if name, rate := selector.Select(game.Event{Kind: game.ShotFired, Sound: "combat-fire", Period: selector.periods["combat-fire"] / 2}); name != "combat-fire" || rate != 2 {
		t.Fatalf("combat shot ignored its recovered period: %s, %g", name, rate)
	}
	if name, _ := selector.Select(game.Event{Kind: game.LifeLost, Value: 0}); name != "" {
		t.Fatal("ordinary life bookkeeping replayed the destruction sound")
	}
	if name, _ := selector.Select(game.Event{Kind: game.PaddleDestroyed}); name != "game-over" {
		t.Fatal("paddle destruction lost its original sample")
	}
	if name, _ := selector.Select(game.Event{Kind: game.LevelStarted, Sound: "missing-source"}); name != "" {
		t.Fatal("an unknown explicit sample was replaced by an unrelated fallback")
	}
}

func TestSourceChannelOrderAndStereoWiring(t *testing.T) {
	channel := 1
	for _, expected := range []int{1, 0, 3, 2, 1, 0} {
		if channel != expected {
			t.Fatalf("Paula channel = %d, expected %d", channel, expected)
		}
		channel = NextPaulaChannel(channel)
	}
	for channel, side := range []int{0, 1, 1, 0} {
		if PaulaStereoSide(channel) != side {
			t.Fatalf("channel %d did not retain its Amiga stereo side", channel)
		}
	}
}

func TestOriginalPCMRetimingAndHardStereoSide(t *testing.T) {
	pcm := make([]byte, 3*4)
	for frame, value := range []int16{1000, 3000, 5000} {
		binary.LittleEndian.PutUint16(pcm[frame*4:], uint16(value))
		binary.LittleEndian.PutUint16(pcm[frame*4+2:], uint16(value))
	}
	resampled := ResampleEffect(pcm, 0.5, 1)
	if len(resampled) != len(pcm)*2 {
		t.Fatal("doubling a Paula period did not double the sample duration")
	}
	for frame, expected := range []int16{1000, 2000, 3000, 4000, 5000, 5000} {
		left := int16(binary.LittleEndian.Uint16(resampled[frame*4:]))
		right := int16(binary.LittleEndian.Uint16(resampled[frame*4+2:]))
		if left != 0 || right != expected {
			t.Fatalf("retimed frame %d = (%d,%d), expected (0,%d)", frame, left, right, expected)
		}
	}
	fast := ResampleEffect(pcm, 2, 3)
	if len(fast) != 8 || int16(binary.LittleEndian.Uint16(fast)) != 1000 || int16(binary.LittleEndian.Uint16(fast[4:])) != 5000 || binary.LittleEndian.Uint16(fast[2:]) != 0 {
		t.Fatal("a faster left-side source did not retain original sample values")
	}
	for _, invalid := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if len(ResampleEffect(pcm, invalid, 0)) != 0 {
			t.Fatal("invalid rate created PCM")
		}
	}
}

func TestOriginalQueueOrderAndOneSoundForBrickDestruction(t *testing.T) {
	events := []game.Event{
		{Kind: game.BrickHit, X: 24, Y: 32},
		{Kind: game.ReserveEarned, Sound: "extra-life"},
		{Kind: game.BrickBreak, X: 24, Y: 32, Sound: "bonus-brick"},
	}
	ordered := OrderedEvents(events)
	if len(ordered) != 2 || ordered[0].Kind != game.BrickBreak || ordered[1].Kind != game.ReserveEarned {
		t.Fatal("source queue was not newest first or duplicated a destruction", ordered)
	}
	if len(OrderedEvents([]game.Event{{Kind: game.BrickHit, Sound: "brick"}})) != 1 {
		t.Fatal("a real reinforced-brick sound was suppressed")
	}
}
