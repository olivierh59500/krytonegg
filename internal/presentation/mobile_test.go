package presentation

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"krytonegg/assets"
	"krytonegg/internal/game"
	"krytonegg/internal/sound"
	"krytonegg/internal/touch"
)

// These checks exercise presentation and save behavior without opening an
// audio device or running the Ebitengine event loop.
func mobileTestApp(t *testing.T) *App {
	t.Helper()
	table, err := assets.Read("original/level.tab")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := assets.Read("levels.json")
	if err != nil {
		t.Fatal(err)
	}
	campaign, err := game.LoadCampaign(table, metadata)
	if err != nil {
		t.Fatal(err)
	}
	return &App{
		world: game.New(campaign, 1990), campaign: campaign, table: table,
		graphics: &Graphics{}, audio: &sound.Audio{}, highScore: 5000,
		options: Options{Mobile: true, Seed: 1990, DataDir: t.TempDir()},
	}
}

func pressMobileButton(a *App, x, y float64) {
	views := a.touchButtons()
	buttons := make([]touch.Button, len(views))
	for i := range views {
		buttons[i] = views[i].Button
	}
	a.touchFrame = a.touchController.Update(
		[]touch.Sample{{ID: 1, X: x, Y: y, Pressed: true}}, buttons,
		touch.Rect{X: 16, Y: 17, W: 288, H: 183}, touch.ModeMenu,
		a.world.Paddle.X, a.world.Paddle.Y,
	)
	a.touchActions()
}

func TestMobileFieldContactDoesNotLaunchAndReleaseIsIdle(t *testing.T) {
	a := mobileTestApp(t)
	a.world.Restart()
	a.touchFrame = touch.Frame{Started: true, Move: true, X: a.world.Paddle.X, Y: a.world.Paddle.Y}
	input := a.input()
	if !input.UseMouse || input.Launch || input.Fire {
		t.Fatalf("field contact became a launch or fire gesture: %+v", input)
	}
	a.world.Tick(input)
	if a.world.State != game.Ready {
		t.Fatal("field contact launched the attached ball")
	}
	a.touchFrame = touch.Frame{}
	input = a.input()
	if input.UseMouse || input.Launch || input.Fire {
		t.Fatalf("released touch retained mouse or firing controls: %+v", input)
	}
	a.touchFrame = touch.Frame{Launch: true, Fire: true}
	a.world.Tick(a.input())
	if a.world.State != game.Playing {
		t.Fatal("fire button did not release the attached ball")
	}
}

func TestMobilePauseReleaseClearsBonusPointerHistory(t *testing.T) {
	for _, bonus := range []game.EffectKind{game.Reverse, game.MouseSensitivity} {
		t.Run(map[game.EffectKind]string{game.Reverse: "reverse", game.MouseSensitivity: "sensitivity"}[bonus], func(t *testing.T) {
			a := mobileTestApp(t)
			a.world.Restart()
			a.world.ApplyBonus(bonus)
			anchor := a.world.Paddle.X
			a.touchFrame = touch.Frame{Started: true, Move: true, X: anchor, Y: a.world.Paddle.Y}
			a.world.Tick(a.input())
			a.touchFrame = touch.Frame{Move: true, X: anchor + 20, Y: a.world.Paddle.Y}
			a.world.Tick(a.input())
			position := a.world.Paddle.X
			if position == anchor+20 {
				t.Fatal("bonus did not establish a distinct virtual pointer position")
			}
			a.Suspend()
			if a.world.State != game.Paused {
				t.Fatal("suspension did not pause the round")
			}
			// Native input is empty before the event loop. The paused update
			// must still discard the released gesture's simulation baseline.
			a.sampleTouches()
			a.world.Tick(a.input())
			a.togglePause()
			a.touchFrame = touch.Frame{Started: true, Move: true, X: position, Y: a.world.Paddle.Y}
			a.world.Tick(a.input())
			if a.world.Paddle.X != position {
				t.Fatalf("new contact after pause jumped from %g to %g", position, a.world.Paddle.X)
			}
		})
	}
}

func TestMobileOverlaySidebarCannotStartHiddenGameplay(t *testing.T) {
	for _, overlay := range []string{"scores", "help"} {
		t.Run(overlay, func(t *testing.T) {
			a := mobileTestApp(t)
			a.scores, a.help = overlay == "scores", overlay == "help"
			pressMobileButton(a, 360, 80)
			if (a.scores || a.help) && a.world.State != game.Title {
				t.Fatalf("sidebar started gameplay behind the %s overlay", overlay)
			}
		})
	}
	a := mobileTestApp(t)
	a.world.Restart()
	a.togglePause()
	a.help, a.helpResume = true, true
	pressMobileButton(a, 360, 20)
	if a.help && a.world.State != game.Paused {
		t.Fatal("sidebar resumed gameplay behind its help overlay")
	}
}

func TestMobileScoresTouchAndBackPreserveTitleNavigation(t *testing.T) {
	a := mobileTestApp(t)
	pressMobileButton(a, 160, 130)
	if !a.scores || a.world.State != game.Title || a.AtTitle() {
		t.Fatal("scores button launched a round or incorrectly allowed Back-to-exit")
	}
	a.Back()
	if a.scores || !a.AtTitle() {
		t.Fatal("Back did not return from scores to the title")
	}
}

func TestMobileSimultaneousTitlePressesCannotStartHiddenGameplay(t *testing.T) {
	a := mobileTestApp(t)
	views := a.touchButtons()
	buttons := make([]touch.Button, len(views))
	for i := range views {
		buttons[i] = views[i].Button
	}
	a.touchFrame = a.touchController.Update([]touch.Sample{
		{ID: 1, X: 160, Y: 80, Pressed: true},
		{ID: 2, X: 160, Y: 130, Pressed: true},
	}, buttons, touch.Rect{W: 320, H: 200}, touch.ModeMenu, 160, 189)
	if !a.touchActions() {
		t.Fatal("title presses did not consume their navigation event")
	}
	if a.scores && a.world.State != game.Title {
		t.Fatal("simultaneous PLAY and SCORES started gameplay behind the score screen")
	}
}

func TestMobileConstructionPaintSaveReloadAndReturnFromTest(t *testing.T) {
	a := mobileTestApp(t)
	a.openEditor()
	c := a.construction
	for i := range c.cells {
		c.cells[i] = 0
	}
	c.bonus = 7
	a.touchFrame = touch.Frame{Painting: true, PaintX: 24, PaintY: 28}
	if err := a.updateEditor(); err != nil {
		t.Fatal(err)
	}
	want := uint16(c.palette[c.selected]) | uint16(c.bonus)<<8
	if c.cells[0] != want {
		t.Fatalf("touch paint did not encode tile and bonus: %04x", c.cells[0])
	}
	a.touchFrame = touch.Frame{Actions: []touch.Action{touch.Save}}
	if err := a.updateEditor(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(a.options.DataDir, "custom.level"))
	if err != nil || len(data) != game.LevelByteSize {
		t.Fatalf("touch save did not write an original-format level: size=%d, err=%v", len(data), err)
	}
	a.touchFrame = touch.Frame{Actions: []touch.Action{touch.Erase}}
	if err := a.updateEditor(); err != nil {
		t.Fatal(err)
	}
	a.touchFrame = touch.Frame{Painting: true, PaintX: 24, PaintY: 28}
	if err := a.updateEditor(); err != nil {
		t.Fatal(err)
	}
	if c.cells[0] != 0 {
		t.Fatal("touch erase did not clear the painted cell")
	}
	a.touchFrame = touch.Frame{Actions: []touch.Action{touch.Load}}
	if err := a.updateEditor(); err != nil {
		t.Fatal(err)
	}
	if c.cells[0] != want {
		t.Fatal("touch reload lost the saved tile or bonus")
	}
	a.touchFrame = touch.Frame{Actions: []touch.Action{touch.Test}}
	if err := a.updateEditor(); err != nil {
		t.Fatal(err)
	}
	if a.editor || !a.testing || a.world.State != game.Ready || len(a.world.Levels) != 1 {
		t.Fatal("touch test did not start the custom level")
	}
	a.Back()
	if !a.editor || a.testing || a.construction != c || len(a.world.Levels) != 60 || c.cells[0] != want {
		t.Fatal("Back from a custom test did not restore editor and campaign")
	}
}

func TestMobileFractionalFitMapsSidebarTouchToItsButton(t *testing.T) {
	g := &Graphics{}
	g.LayoutTouch(2340, 1080)
	if math.Abs(g.Scale-5.4) > 1e-9 || math.Abs(g.OffsetX-90) > 1e-9 || math.Abs(g.OffsetY) > 1e-9 {
		t.Fatalf("phone layout did not uniformly fit the complete field and sidebar: %+v", g)
	}
	x, y := g.WorldPosition(2034, 432)
	if math.Abs(x-360) > 1e-9 || math.Abs(y-80) > 1e-9 {
		t.Fatalf("fitted touch did not map to the native sidebar: %g,%g", x, y)
	}
	a := mobileTestApp(t)
	a.world.Restart()
	pressMobileButton(a, x, y)
	if !a.touchFrame.Launch || !a.touchFrame.Fire || a.touchFrame.Move {
		t.Fatalf("sidebar fire touch was routed to the field: %+v", a.touchFrame)
	}
}
