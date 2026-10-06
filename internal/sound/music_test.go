package sound

import (
	"bytes"
	"math"
	"testing"

	zikmu "github.com/olivierh59500/go-zikmu"

	"krytonegg/assets"
)

// This test exercises the real disk modules without opening an audio device.
func TestOriginalModulesRenderInPureGo(t *testing.T) {
	for _, name := range []string{"intro", "halloffame"} {
		t.Run(name, func(t *testing.T) {
			data, err := assets.Read("original/" + name)
			if err != nil {
				t.Fatal(err)
			}
			module, err := zikmu.Load(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			if module.Metadata.Channels != 4 {
				t.Fatalf("expected 4 Amiga channels, got %d", module.Metadata.Channels)
			}
			player, err := zikmu.NewPlayer(module, zikmu.DefaultConfig())
			if err != nil {
				t.Fatal(err)
			}
			pcm := make([]float32, 44100*2)
			n, err := player.Render(pcm)
			if err != nil {
				t.Fatal(err)
			}
			var energy float64
			for _, v := range pcm[:n] {
				if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
					t.Fatal("invalid PCM")
				}
				energy += float64(v) * float64(v)
			}
			if energy < 1 {
				t.Fatalf("original module produced silence: energy %f", energy)
			}
		})
	}
}
