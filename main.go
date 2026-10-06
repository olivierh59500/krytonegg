// Krypton Egg is a native Go/Ebitengine port of the supplied Amiga disk.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	"krytonegg/internal/presentation"
)

func main() {
	options := presentation.Options{}
	flag.IntVar(&options.Scale, "scale", 4, "initial integer zoom of the original 320x200 frame")
	flag.IntVar(&options.StartLevel, "level", 0, "start at an original round (1-60); 0 opens the title")
	flag.IntVar(&options.StartCombat, "combat", 0, "start at an original alien encounter (1-6)")
	flag.Uint64Var(&options.Seed, "seed", 1990, "deterministic simulation seed")
	flag.BoolVar(&options.Muted, "mute", false, "start with sound muted")
	flag.BoolVar(&options.Fullscreen, "fullscreen", false, "start in fullscreen")
	flag.BoolVar(&options.Editor, "editor", false, "open the construction set")
	flag.StringVar(&options.CustomLevel, "custom", "", "play an original-format 576-byte custom level")
	flag.StringVar(&options.DataDir, "data-dir", "", "directory for local high scores and custom levels")
	flag.IntVar(&options.SmokeTicks, "smoke", 0, "run an automatic check and exit after this many updates")
	flag.StringVar(&options.Capture, "capture", "", "save the final smoke-check frame as a PNG")
	flag.Parse()
	if options.Scale < 1 || options.Scale > 12 {
		log.Fatal("scale must be between 1 and 12")
	}
	if options.StartLevel < 0 || options.StartLevel > 60 {
		log.Fatal("level must be between 1 and 60, or 0 for the title")
	}
	if options.StartCombat < 0 || options.StartCombat > 6 {
		log.Fatal("combat must be between 1 and 6, or 0 for the campaign")
	}
	if options.StartCombat != 0 && (options.StartLevel != 0 || options.CustomLevel != "" || options.Editor) {
		log.Fatal("combat cannot be combined with level, custom, or editor")
	}
	if options.SmokeTicks < 0 {
		log.Fatal("smoke update count must not be negative")
	}
	if options.Capture != "" && options.SmokeTicks == 0 {
		log.Fatal("capture requires a positive smoke update count")
	}
	if options.DataDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			base = "."
		}
		options.DataDir = filepath.Join(base, "krytonegg")
	}
	app, err := presentation.New(options)
	if err != nil {
		log.Fatal(err)
	}
	defer app.Close()
	ebiten.SetWindowTitle("Krypton Egg — Go / Ebitengine")
	ebiten.SetWindowSize(320*options.Scale, 200*options.Scale)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetFullscreen(options.Fullscreen)
	ebiten.SetScreenFilterEnabled(false)
	ebiten.SetTPS(50)
	if err := ebiten.RunGame(app); err != nil {
		log.Fatal(err)
	}
	if options.SmokeTicks > 0 {
		fmt.Println(app.CheckReport())
	}
}
