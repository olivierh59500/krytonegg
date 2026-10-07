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
	flag.BoolVar(&options.Mobile, "touch", false, "preview the Android touch controls on desktop")
	flag.BoolVar(&options.Expert, "expert", false, "use a bounded simulated expert player")
	flag.BoolVar(&options.Showcase, "showcase", false, "run the four-minute presentation with options and genuine expert play")
	flag.StringVar(&options.Movie, "movie", "", "record actual Ebitengine frames and offline original audio to an MP4")
	flag.IntVar(&options.MovieTicks, "movie-ticks", 0, "record this many PAL updates; movie default is 12000 (four minutes)")
	flag.Float64Var(&options.PlayerSpeed, "player-speed", 10, "simulated player's maximum native pixels per PAL update")
	flag.IntVar(&options.PlayerReaction, "player-reaction", 5, "simulated player's observation delay in PAL updates")
	flag.StringVar(&options.CustomLevel, "custom", "", "play an original-format 576-byte custom level")
	flag.StringVar(&options.DataDir, "data-dir", "", "directory for local high scores and custom levels")
	flag.IntVar(&options.SmokeTicks, "smoke", 0, "run an automatic check and exit after this many updates")
	flag.StringVar(&options.Capture, "capture", "", "save the final smoke-check frame as a PNG")
	flag.Parse()
	if options.PlayerSpeed <= 0 || options.PlayerSpeed > 30 || options.PlayerReaction < 1 || options.PlayerReaction > 50 {
		log.Fatal("player speed must be in (0,30] and reaction delay in [1,50] PAL updates")
	}
	if options.Showcase {
		options.Expert = true
	}
	if options.Expert && options.SmokeTicks > 0 {
		log.Fatal("expert playback and smoke input are separate validation modes")
	}
	if options.Movie != "" && options.MovieTicks == 0 {
		options.MovieTicks = 12000
	}
	if options.Movie != "" {
		options.Muted = true
	}
	if options.MovieTicks < 0 || (options.MovieTicks > 0 && options.Movie == "") {
		log.Fatal("movie-ticks requires a movie path and a nonnegative update count")
	}
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
	ebiten.SetWindowTitle("Krypton Egg — Go / Ebitengine")
	windowWidth := 320
	if options.Mobile {
		windowWidth = 400
	}
	ebiten.SetWindowSize(windowWidth*options.Scale, 200*options.Scale)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetFullscreen(options.Fullscreen)
	ebiten.SetScreenFilterEnabled(false)
	if options.Expert || options.Showcase || options.Movie != "" {
		ebiten.SetRunnableOnUnfocused(true)
	}
	ebiten.SetTPS(50)
	runError := ebiten.RunGame(app)
	app.Close()
	if runError != nil {
		log.Fatal(runError)
	}
	if err := app.ExportError(); err != nil {
		log.Fatal(err)
	}
	if options.SmokeTicks > 0 {
		fmt.Println(app.CheckReport())
	}
}
