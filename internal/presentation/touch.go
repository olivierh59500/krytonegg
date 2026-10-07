package presentation

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"krytonegg/internal/game"
	"krytonegg/internal/touch"
)

type touchButton struct {
	touch.Button
	label string
}

const touchCredits touch.Action = touch.Back + 1

func button(action touch.Action, label string, x, y, w, h float64) touchButton {
	return touchButton{Button: touch.Button{Action: action, Bounds: touch.Rect{X: x, Y: y, W: w, H: h}}, label: label}
}

func (a *App) layout(width, height int) {
	if a.options.Mobile {
		a.graphics.LayoutTouch(width, height)
	} else {
		a.graphics.Layout(width, height)
	}
}

// The sidebar lies outside the original 320x200 field, so it never covers a ball.
func (a *App) touchButtons() []touchButton {
	if a.introActive() || a.nameActive() || a.interludeActive() || a.levelBanner {
		label := "NEXT"
		if a.nameActive() {
			label = "OK"
		}
		return []touchButton{
			button(touch.Back, "BACK", 324, 8, 72, 36), button(touch.Fire, label, 324, 52, 72, 56),
			button(touch.Sound, "SOUND", 324, 116, 72, 36), button(touch.Menu, "MENU", 324, 160, 72, 36),
		}
	}
	if a.help || a.scores || a.credits {
		return []touchButton{
			button(touch.Back, "BACK", 324, 8, 72, 36), button(touch.Sound, "SOUND", 324, 116, 72, 36),
			button(touch.Menu, "MENU", 324, 160, 72, 36), button(touch.Back, "", 0, 0, 320, 200),
		}
	}
	if a.editor {
		return []touchButton{
			button(touch.TilePrev, "PREV", 324, 4, 34, 26), button(touch.TileNext, "NEXT", 362, 4, 34, 26),
			button(touch.Bonus, "BONUS", 324, 34, 34, 26), button(touch.Power, "POWER", 362, 34, 34, 26),
			button(touch.Erase, "ERASE", 324, 64, 72, 26),
			button(touch.Save, "SAVE", 324, 94, 34, 26), button(touch.Load, "LOAD", 362, 94, 34, 26),
			button(touch.Test, "TEST", 324, 124, 72, 26), button(touch.Back, "BACK", 324, 154, 72, 26),
		}
	}
	pauseLabel, fireLabel := "PAUSE", "FIRE"
	pauseAction, fireAction := touch.Pause, touch.Fire
	if a.world.State == game.Paused {
		pauseLabel = "RESUME"
		fireLabel = "RESUME"
	}
	if a.world.State == game.Title {
		pauseLabel = "HELP"
		pauseAction = touch.Help
		fireLabel = "PLAY"
		fireAction = touch.Play
	}
	if a.world.State == game.GameOver || a.world.State == game.Won {
		fireLabel = "PLAY"
		fireAction = touch.Play
	}
	buttons := []touchButton{
		button(pauseAction, pauseLabel, 324, 8, 72, 36), button(fireAction, fireLabel, 324, 52, 72, 56),
		button(touch.Sound, "SOUND", 324, 116, 72, 36), button(touch.Menu, "MENU", 324, 160, 72, 36),
	}
	if a.world.State == game.Title {
		buttons = append(buttons,
			button(touch.Play, "", 96, 64, 128, 32), button(touch.Scores, "", 96, 116, 128, 29),
			button(touch.Editor, "", 16, 145, 144, 24), button(touch.Help, "", 160, 145, 144, 24),
			button(touch.RoundPrev, "PREV", 16, 98, 48, 32), button(touch.RoundNext, "NEXT", 256, 98, 48, 32),
			button(touchCredits, "", 40, 165, 240, 35),
		)
	} else if a.world.State == game.GameOver || a.world.State == game.Won {
		buttons = append(buttons, button(touch.Play, "", 0, 112, 320, 88))
	}
	return buttons
}

func (a *App) sampleTouches() {
	a.touchIDs = ebiten.AppendTouchIDs(a.touchIDs[:0])
	a.touchSamples = a.touchSamples[:0]
	for _, id := range a.touchIDs {
		x, y := ebiten.TouchPosition(id)
		wx, wy := a.graphics.WorldPosition(x, y)
		a.touchSamples = append(a.touchSamples, touch.Sample{ID: int(id), X: wx, Y: wy, Pressed: inpututil.TouchPressDuration(id) == 1})
	}
	// A mouse contact makes -touch a useful desktop preview of the exact Android UI.
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		wx, wy := a.graphics.WorldPosition(x, y)
		a.touchSamples = append(a.touchSamples, touch.Sample{ID: -1, X: wx, Y: wy, Pressed: inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)})
	}
	mode := touch.ModeDrag
	if a.world.State == game.Title || a.world.State == game.GameOver || a.world.State == game.Won || a.world.State == game.Paused || a.world.State == game.LevelClear || a.help || a.scores {
		mode = touch.ModeMenu
	}
	if a.editor {
		mode = touch.ModePaint
	}
	if a.help || a.scores || a.credits || a.introActive() || a.nameActive() || a.interludeActive() || a.levelBanner {
		mode = touch.ModeMenu
	}
	x, y := a.world.Paddle.X, a.world.Paddle.Y
	if a.world.Combat != nil {
		x, y = a.world.Combat.ShipX, a.world.Combat.ShipY
	}
	views := a.touchButtons()
	buttons := make([]touch.Button, len(views))
	for i := range views {
		buttons[i] = views[i].Button
	}
	a.touchFrame = a.touchController.Update(a.touchSamples, buttons, touch.Rect{X: 16, Y: 17, W: 288, H: 183}, mode, x, y)
	if a.touchFrame.Started || !a.touchFrame.Move {
		a.world.ResetPointer()
	}
}

func (a *App) touchActions() bool {
	consumed := false
	for _, action := range a.touchFrame.Actions {
		switch action {
		case touch.Fire:
		case touch.Sound:
			a.audio.ToggleMute()
		case touch.Pause:
			a.togglePause()
			consumed = true
		case touch.Menu:
			a.saveScore()
			a.returnToTitle()
			consumed = true
		case touch.Back:
			a.Back()
			consumed = true
		case touch.Help:
			a.help = !a.help
			if a.help && (a.world.State == game.Playing || a.world.State == game.Ready || a.world.State == game.Combat) {
				a.togglePause()
				a.helpResume = true
			}
			a.audio.SetPaused(a.help || a.world.State == game.Paused)
			consumed = true
		case touch.Scores:
			a.scores = true
			a.audio.Music("halloffame")
			consumed = true
		case touchCredits:
			a.credits = true
			consumed = true
		case touch.Editor:
			a.openEditor()
			consumed = true
		case touch.RoundPrev:
			a.selectedRound = max(0, a.selectedRound-1)
		case touch.RoundNext:
			a.selectedRound = min(len(a.campaign)-1, a.selectedRound+1)
		case touch.Play:
			if a.world.State == game.Title {
				_ = a.world.StartAt(a.selectedRound)
			} else {
				a.world.Restart()
			}
			a.runScoreRecorded = false
			a.audio.Music("")
			a.consumeEvents()
			consumed = true
		}
		// Button ownership was resolved against the previous screen. Apply one
		// navigation transition per frame before considering its new controls.
		if consumed {
			return true
		}
	}
	return consumed
}

func (a *App) togglePause() {
	state := a.world.State
	if state == game.Playing || state == game.Ready || state == game.Combat || state == game.Paused {
		a.world.Tick(game.Input{Pause: true})
		a.audio.SetPaused(a.world.State == game.Paused)
	}
}

// Suspend runs on the game thread after Android suspends its view. Progress is
// persisted, and the player explicitly resumes an interrupted round.
func (a *App) Suspend() {
	a.saveScore()
	if a.world.State == game.Playing || a.world.State == game.Ready || a.world.State == game.Combat {
		a.togglePause()
	}
}

// Back closes overlays and tests, then pauses gameplay or returns to the title.
func (a *App) Back() {
	if a.nameActive() {
		if err := a.presentation.FinishName(); err != nil {
			a.message, a.messageTicks = "SCORE SAVE FAILED", 180
		}
		a.highScore = a.presentation.Table.Best()
		a.scores = true
		return
	}
	if a.introActive() {
		a.presentation.SkipIntro()
		return
	}
	if a.interludeActive() {
		a.presentation.Interlude.Active = false
		a.offerRunScore()
		return
	}
	if a.help {
		a.help = false
		if a.helpResume {
			a.togglePause()
			a.helpResume = false
		}
		a.audio.SetPaused(a.world.State == game.Paused)
		return
	}
	if a.scores || a.credits {
		a.closeOverlay()
		return
	}
	if a.testing {
		a.testing, a.editor = false, true
		a.resetWorld()
		a.audio.Music("")
		return
	}
	if a.editor {
		a.returnToTitle()
		return
	}
	if a.world.State == game.Playing || a.world.State == game.Ready || a.world.State == game.Combat {
		a.Suspend()
		return
	}
	a.saveScore()
	a.returnToTitle()
}

// AtTitle supports Android's standard Back-to-exit behavior without sharing game state.
func (a *App) AtTitle() bool {
	return a.world.State == game.Title && !a.editor && !a.scores && !a.help && !a.credits && !a.introActive() && !a.nameActive() && !a.interludeActive()
}

// VerificationDone allows an Android check to hold a frame rather than exit its view.
func (a *App) VerificationDone() bool {
	return a.options.SmokeTicks > 0 && a.steps >= a.options.SmokeTicks
}

func (a *App) drawTouchControls(screen *ebiten.Image) {
	g := a.graphics
	for _, view := range a.touchButtons() {
		if view.label == "" {
			continue
		}
		r := view.Bounds
		// The control bezel is an enlarged original brick, not a new bitmap.
		g.draw(screen, g.images["sprites/brick-1.png"], r.X, r.Y, r.W/16, r.H/8)
		g.Clear(screen, r.X+3, r.Y+3, r.W-6, r.H-6)
		textScale := math.Min(1, (r.W-8)/float64(len(view.label)*8))
		x, y := r.X+(r.W-float64(len(view.label)*8)*textScale)/2, r.Y+(r.H-8*textScale)/2
		for _, letter := range view.label {
			if letter >= 'A' && letter <= 'Z' {
				g.draw(screen, g.font[letter-'A'], x, y, textScale, textScale)
			}
			x += 8 * textScale
		}
		if view.Action == touch.Sound && a.audio.Muted() {
			g.Text(screen, "OFF", r.X+24, r.Y+r.H-11)
		}
		if view.Action == touch.Erase && a.touchErase {
			g.Text(screen, "ON", r.X+28, r.Y+r.H-11)
		}
	}
}

func (a *App) editorTouchInfo() string {
	mode := "PAINT"
	if a.touchErase {
		mode = "ERASE"
	}
	return fmt.Sprintf("%s TILE %03d BONUS %02d POWER %d", mode, a.construction.palette[a.construction.selected], a.construction.bonus>>2, a.construction.bonus&3)
}
