package presentation

import (
	"encoding/json"
	"fmt"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"krytonegg/assets"
	"krytonegg/internal/game"
	"krytonegg/internal/player"
	"krytonegg/internal/recording"
	"krytonegg/internal/sound"
	"krytonegg/internal/touch"
)

// Options controls the native window and optional reproducible development checks.
type Options struct {
	Scale, StartLevel, StartCombat, SmokeTicks int
	Seed                                       uint64
	Muted, Fullscreen, Editor                  bool
	Mobile, FreezeCheck                        bool
	CustomLevel, DataDir, Capture              string
	Expert, Showcase                           bool
	Movie                                      string
	MovieTicks                                 int
	PlayerSpeed                                float64
	PlayerReaction                             int
}

// App translates native input to the simulation and draws original bitmaps.
type App struct {
	world                           *game.World
	graphics                        *Graphics
	audio                           *sound.Audio
	options                         Options
	table                           []byte
	campaign                        []game.Level
	combatBoundary                  []int
	steps, highScore, selectedRound int
	mouseX, mouseY                  int
	mouseMode                       bool
	help, editor, testing           bool
	scores, menuScores, helpResume  bool
	construction                    *Construction
	capturePending, captured        bool
	captureError                    error
	touchIDs                        []ebiten.TouchID
	gamepadIDs                      []ebiten.GamepadID
	message                         string
	messageTicks                    int
	touchController                 touch.Controller
	touchSamples                    []touch.Sample
	touchFrame                      touch.Frame
	touchErase                      bool
	presentation                    *Presentation
	pilot                           *player.Agent
	recorder                        *recording.Recorder
	showcase                        *Showcase
	credits, runScoreRecorded       bool
	movieDrawnStep                  int
	moviePixels                     []byte
	stepEvents                      []game.Event
	textureCursor, displayIndex     int
	pendingLevelStart               *game.Event
	levelBanner                     bool
	levelBannerTicks                int
	CheatAuthorized                 bool
}

// New initializes the fully native game without launching an emulator.
func New(options Options) (*App, error) {
	table, err := assets.Read("original/level.tab")
	if err != nil {
		return nil, err
	}
	metadata, err := assets.Read("levels.json")
	if err != nil {
		return nil, err
	}
	levels, err := game.LoadCampaign(table, metadata)
	if err != nil {
		return nil, err
	}
	combatData, err := assets.Read("combat.json")
	if err != nil {
		return nil, err
	}
	var combatMetadata struct {
		Boundary []int `json:"boss_boundary_by_4px_row"`
	}
	if err := json.Unmarshal(combatData, &combatMetadata); err != nil {
		return nil, err
	}
	if options.CustomLevel != "" {
		custom, err := os.ReadFile(options.CustomLevel)
		if err != nil {
			return nil, err
		}
		if len(custom) != game.LevelByteSize {
			return nil, fmt.Errorf("custom level must contain exactly %d bytes", game.LevelByteSize)
		}
		levels, err = game.LoadLevels(custom)
		if err != nil {
			return nil, err
		}
	}
	graphics, err := NewGraphics()
	if err != nil {
		return nil, err
	}
	graphics.SetStars(options.Seed)
	controller, err := NewPresentation(options.DataDir)
	if err != nil {
		return nil, err
	}
	// Automated reviews render their soundtrack offline and stay silent locally.
	muted := options.Muted || options.Expert || options.Showcase || options.SmokeTicks > 0 || options.Movie != ""
	audio, err := sound.New(muted)
	if err != nil {
		return nil, err
	}
	a := &App{world: game.New(levels, options.Seed), campaign: levels, combatBoundary: combatMetadata.Boundary, graphics: graphics, audio: audio, table: table, options: options, mouseMode: true, presentation: controller, highScore: controller.Table.Best()}
	a.world.SetCombatBoundary(a.combatBoundary)
	if controller.ScoreError != nil {
		a.message, a.messageTicks = "SCORE LOAD FAILED", 180
	}
	if options.Expert || options.Showcase || options.Movie != "" {
		a.pilot = a.newPilot()
	}
	if options.StartLevel > 0 || options.StartCombat > 0 || options.CustomLevel != "" || options.Editor || options.SmokeTicks > 0 {
		controller.SkipIntro()
	}
	if options.StartLevel > 0 {
		if err := a.world.StartAt(options.StartLevel - 1); err != nil {
			a.Close()
			return nil, err
		}
		a.selectedRound = options.StartLevel - 1
	}
	if options.CustomLevel != "" {
		a.world.Restart()
	}
	if options.StartCombat > 0 {
		if err := a.world.StartCombat(options.StartCombat); err != nil {
			a.Close()
			return nil, err
		}
	}
	if options.Editor {
		a.openEditor()
	}
	if options.Movie != "" {
		a.recorder, err = recording.New(options.Movie, 1280, 800, 50, 50)
		if err != nil {
			a.Close()
			return nil, err
		}
		a.moviePixels = make([]byte, 1280*800*4)
	}
	if options.Showcase {
		a.showcase = &Showcase{}
	}
	a.layout(game.Width*options.Scale, game.Height*options.Scale)
	return a, nil
}

func just(key ebiten.Key) bool { return inpututil.IsKeyJustPressed(key) }

func (a *App) key(key ebiten.Key) bool { return !a.automatic() && just(key) }

func (a *App) automatic() bool {
	return a.options.SmokeTicks > 0 || a.options.Expert || a.options.Showcase || a.options.Movie != ""
}

func (a *App) introActive() bool {
	return a.presentation != nil && a.presentation.Intro.Phase != IntroFinished
}

func (a *App) nameActive() bool { return a.presentation != nil && a.presentation.Name.Active }

func (a *App) interludeActive() bool {
	return a.presentation != nil && a.presentation.Interlude.Active
}

// Update is the sole owner of simulation advances and input edges.
func (a *App) Update() (err error) {
	a.enforceAutomaticMode()
	if a.captureError != nil {
		return a.captureError
	}
	if a.recorder != nil && a.recorder.Due() {
		return nil
	}
	previousStep := a.steps
	a.stepEvents = a.stepEvents[:0]
	if previousStep == 0 && a.world.State != game.Title {
		a.consumeEvents()
	}
	defer func() {
		a.audio.Music(a.currentMusic())
		a.audio.SetPaused(a.help || a.world.State == game.Paused)
		if err == nil && a.recorder != nil && a.steps != previousStep {
			err = a.recorder.Advance(a.stepEvents, a.currentMusic(), a.help || a.world.State == game.Paused)
		}
	}()
	return a.updateFrame()
}

func (a *App) currentMusic() string {
	if a.introActive() {
		return "intro"
	}
	if a.interludeActive() || a.editor {
		return ""
	}
	if a.scores || a.credits || a.nameActive() || a.world.State == game.GameOver || a.world.State == game.Won {
		return "halloffame"
	}
	if a.world.State == game.Title {
		return "intro"
	}
	return ""
}

func (a *App) updateFrame() error {
	if a.captureError != nil {
		return a.captureError
	}
	if a.options.MovieTicks > 0 && a.steps >= a.options.MovieTicks {
		return ebiten.Termination
	}
	if a.options.SmokeTicks > 0 && a.steps >= a.options.SmokeTicks {
		if a.options.FreezeCheck {
			return nil
		}
		if a.options.Capture == "" || a.captured {
			return ebiten.Termination
		}
		a.capturePending = true
		return nil
	}
	a.steps++
	if a.messageTicks > 0 {
		a.messageTicks--
	}
	if a.showcase != nil && a.showcase.Update(a) {
		return nil
	}
	if a.options.Mobile && !a.automatic() {
		a.sampleTouches()
	}
	if a.updateOriginalCodes() {
		return nil
	}
	if a.key(ebiten.KeyF11) || a.key(ebiten.KeyF) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	if a.key(ebiten.KeyM) {
		a.audio.ToggleMute()
	}
	if a.key(ebiten.KeyF1) && a.recorder == nil {
		if ebiten.TPS() == 50 {
			ebiten.SetTPS(60)
		} else {
			ebiten.SetTPS(50)
		}
	}
	if a.options.Mobile && (a.introActive() || a.nameActive() || a.interludeActive()) {
		for _, action := range a.touchFrame.Actions {
			if action == touch.Sound {
				a.audio.ToggleMute()
			}
		}
	}
	if a.introActive() {
		fire := a.modalFire()
		if a.options.Mobile {
			for _, sample := range a.touchSamples {
				if sample.Pressed && sample.X >= 324 && ((sample.Y >= 8 && sample.Y < 44) || (sample.Y >= 160 && sample.Y < 196)) {
					a.presentation.SkipIntro()
					return nil
				}
			}
		}
		if a.pilot != nil && !a.options.Showcase && a.presentation.Intro.Phase == IntroWait && a.presentation.Intro.Ticks >= 200 {
			fire = true
		}
		a.presentation.Intro.Update(fire)
		return nil
	}
	if a.nameActive() {
		a.updateNameEntry()
		return nil
	}
	if a.interludeActive() {
		fire := a.modalFire() || (a.pilot != nil && a.presentation.Interlude.Ticks >= 200)
		a.presentation.UpdateInterlude(fire)
		if !a.interludeActive() {
			if a.pendingLevelStart != nil {
				a.audio.Event(*a.pendingLevelStart)
				a.stepEvents = append(a.stepEvents, *a.pendingLevelStart)
				a.pendingLevelStart = nil
			}
			a.offerRunScore()
		}
		return nil
	}
	if a.levelBanner {
		a.levelBannerTicks++
		if a.modalFire() || a.options.SmokeTicks > 0 || (a.pilot != nil && a.levelBannerTicks >= 50) {
			a.levelBanner = false
		}
		return nil
	}
	if a.options.Mobile && !a.automatic() && a.touchActions() {
		return nil
	}
	if a.key(ebiten.KeyH) {
		a.help = !a.help
		if a.help && (a.world.State == game.Playing || a.world.State == game.Ready || a.world.State == game.Combat) {
			a.togglePause()
			a.helpResume = true
		} else if !a.help && a.helpResume {
			a.togglePause()
			a.helpResume = false
		}
	}
	if a.key(ebiten.KeyF3) {
		score, eligible := a.world.Score, !a.testing && a.world.State != game.Title
		a.returnToTitle()
		if eligible && a.presentation != nil {
			a.runScoreRecorded = true
			a.presentation.BeginName(score)
			a.scores = true
		}
		return nil
	}
	if a.key(ebiten.KeyQ) && (a.world.State == game.Title || a.world.State == game.Paused) {
		return ebiten.Termination
	}
	if a.help {
		return nil
	}
	if a.key(ebiten.KeyEscape) {
		a.Back()
		return nil
	}
	if a.scores || a.credits {
		if a.modalFire() {
			a.closeOverlay()
		}
		return nil
	}
	if a.editor {
		if a.options.SmokeTicks > 0 {
			return nil
		}
		return a.updateEditor()
	}
	a.offerRunScore()
	if a.nameActive() || a.scores {
		return nil
	}
	if a.world.State == game.Title {
		if a.key(ebiten.KeyE) {
			a.openEditor()
			return nil
		}
		if a.key(ebiten.KeyC) {
			a.credits = true
			return nil
		}
		if a.key(ebiten.KeyArrowLeft) {
			a.selectedRound = max(0, a.selectedRound-1)
		}
		if a.key(ebiten.KeyArrowRight) {
			a.selectedRound = min(len(a.world.Levels)-1, a.selectedRound+1)
		}
		if a.key(ebiten.KeyArrowDown) || a.key(ebiten.KeyArrowUp) {
			a.menuScores = !a.menuScores
		}
	}
	input := game.Input{}
	if a.pilot != nil {
		input = a.pilot.Next(a.world)
	} else {
		input = a.input()
	}
	if a.options.SmokeTicks > 0 {
		input = game.Input{UseMouse: true, PaddleX: a.world.Paddle.X, PaddleY: 190, Fire: true}
		if len(a.world.Balls) > 0 {
			input.PaddleX = a.world.Balls[0].X
		}
		input.Launch = a.steps > 1 && (a.world.State == game.Title || a.world.State == game.Ready || a.world.State == game.LevelClear)
		if a.world.Combat != nil {
			input.PaddleY = 70
		}
	}
	previousState := a.world.State
	if a.world.State == game.Title && input.Launch {
		selectedScores := a.menuScores
		if !a.automatic() && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := a.graphics.WorldPosition(a.mouseX, a.mouseY)
			selectedScores = x >= 96 && x <= 224 && y >= 116 && y <= 145
			if y >= 165 && x >= 40 && x <= 280 {
				a.credits = true
				return nil
			}
		}
		if selectedScores {
			a.scores = true
			return nil
		}
		if err := a.world.StartAt(a.selectedRound); err != nil {
			return err
		}
		a.runScoreRecorded = false
	} else {
		a.world.Tick(input)
		if (previousState == game.GameOver || previousState == game.Won) && a.world.State == game.Ready {
			a.runScoreRecorded = false
		}
	}
	a.consumeEvents()
	if a.world.Score > a.highScore {
		a.highScore = a.world.Score
	}
	if previousState == game.Combat && a.world.State == game.GameOver && a.presentation != nil && a.world.Combat != nil {
		a.presentation.BeginInterlude(a.world.Combat.Number, true)
	}
	if !a.interludeActive() {
		a.offerRunScore()
	}

	return nil
}

func (a *App) modalFire() bool {
	if a.key(ebiten.KeySpace) || a.key(ebiten.KeyEnter) || a.key(ebiten.KeyF2) || a.gamepadConfirm() {
		return true
	}
	if !a.automatic() && !a.options.Mobile && (inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)) {
		return true
	}
	if a.options.Mobile && !a.automatic() {
		for _, sample := range a.touchSamples {
			if sample.Pressed && !(sample.X >= 320 && sample.Y >= 116 && sample.Y < 152) {
				return true
			}
		}
	}
	return false
}

// gamepadConfirm uses the same standard bottom-button edge as gameplay launch.
// Automated review modes ignore connected controllers throughout every modal.
func (a *App) gamepadConfirm() bool {
	if a.automatic() {
		return false
	}
	a.gamepadIDs = ebiten.AppendGamepadIDs(a.gamepadIDs[:0])
	for _, id := range a.gamepadIDs {
		if ebiten.IsStandardGamepadLayoutAvailable(id) && inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonRightBottom) {
			return true
		}
	}
	return false
}

func (a *App) updateNameEntry() {
	entry := &a.presentation.Name
	returnToTitle := false
	confirm := a.key(ebiten.KeyEnter) || a.key(ebiten.KeyF2) || a.key(ebiten.KeyEscape) || a.gamepadConfirm()
	if !a.automatic() {
		entry.Type(string(ebiten.AppendInputChars(nil)))
		if a.key(ebiten.KeyBackspace) || a.key(ebiten.KeyDelete) {
			entry.Backspace()
		}
		if a.options.Mobile {
			for _, sample := range a.touchSamples {
				if !sample.Pressed {
					continue
				}
				confirm = entry.Touch(sample.X, sample.Y) || confirm
				if sample.X >= 324 && sample.Y >= 52 && sample.Y < 108 {
					confirm = true
				}
				if sample.X >= 324 && sample.Y >= 8 && sample.Y < 44 {
					confirm = true
				}
				if sample.X >= 324 && sample.Y >= 160 && sample.Y < 196 {
					confirm, returnToTitle = true, true
				}
			}
		} else if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := ebiten.CursorPosition()
			wx, wy := a.graphics.WorldPosition(x, y)
			confirm = entry.Touch(wx, wy) || confirm
		}
	}
	if a.pilot != nil && !a.options.Showcase {
		entry.Type("EXPERT PLAYER")
		confirm = true
	}
	if confirm {
		if err := a.presentation.FinishName(); err != nil {
			a.message, a.messageTicks = "SCORE SAVE FAILED", 180
		}
		a.highScore = a.presentation.Table.Best()
		a.scores = true
		if returnToTitle {
			a.returnToTitle()
		}
	}
}

func (a *App) offerRunScore() {
	if a.runScoreRecorded || a.testing || a.presentation == nil || (a.world.State != game.GameOver && a.world.State != game.Won) {
		return
	}
	a.runScoreRecorded = true
	if a.presentation.BeginName(a.world.Score) {
		a.scores = true
	}
}

func (a *App) consumeEvents() {
	var audioBatch []game.Event
	for _, event := range a.world.Events {
		if event.Kind == game.LevelStarted && a.interludeActive() {
			pending := event
			a.pendingLevelStart = &pending
		} else {
			a.stepEvents = append(a.stepEvents, event)
			audioBatch = append(audioBatch, event)
		}
		if event.Kind == game.LevelStarted {
			a.levelBanner, a.levelBannerTicks = a.options.SmokeTicks == 0, 0
			a.displayIndex = a.textureCursor % 83
			a.graphics.BackgroundIndex = a.displayIndex
			a.textureCursor = (a.textureCursor + 1) % 83
		}
		if event.Kind == game.CombatCompleted && a.presentation != nil {
			a.presentation.BeginInterlude(event.Value, false)
		}
	}
	a.audio.Events(audioBatch)
}

func (a *App) closeOverlay() {
	a.scores, a.credits, a.menuScores = false, false, false
	if a.world.State == game.GameOver || a.world.State == game.Won {
		a.returnToTitle()
	}
}

func (a *App) input() game.Input {
	mx, my := ebiten.CursorPosition()
	left := ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA)
	right := ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD)
	up := ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyW)
	down := ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS)
	if left || right || up || down {
		a.mouseMode = false
	}
	if mx != a.mouseX || my != a.mouseY {
		a.mouseMode = true
	}
	a.mouseX, a.mouseY = mx, my
	x, y := a.graphics.WorldPosition(mx, my)
	launch := a.key(ebiten.KeyF2) || a.key(ebiten.KeyEnter) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if a.world.State == game.Title || a.world.State == game.GameOver || a.world.State == game.Won {
		launch = launch || a.key(ebiten.KeySpace)
	}
	fire := ebiten.IsKeyPressed(ebiten.KeyF2) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	if !a.options.Mobile {
		a.touchIDs = ebiten.AppendTouchIDs(a.touchIDs[:0])
	}
	if !a.options.Mobile && len(a.touchIDs) > 0 {
		tx, ty := ebiten.TouchPosition(a.touchIDs[0])
		x, y = a.graphics.WorldPosition(tx, ty)
		a.mouseMode = true
		launch = launch || inpututil.TouchPressDuration(a.touchIDs[0]) == 1
		fire = true
	}
	a.gamepadIDs = ebiten.AppendGamepadIDs(a.gamepadIDs[:0])
	gamepadLaunch, gamepadFire := false, false
	for _, id := range a.gamepadIDs {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		axis := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		vertical := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		if axis < -0.2 {
			left, a.mouseMode = true, false
		}
		if axis > 0.2 {
			right, a.mouseMode = true, false
		}
		if vertical < -0.2 {
			up, a.mouseMode = true, false
		}
		if vertical > 0.2 {
			down, a.mouseMode = true, false
		}
		gamepadLaunch = gamepadLaunch || inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonRightBottom)
		gamepadFire = gamepadFire || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightBottom)
	}
	launch = launch || gamepadLaunch
	fire = fire || gamepadFire
	pause := a.key(ebiten.KeyP)
	if a.world.State == game.Playing || a.world.State == game.Ready || a.world.State == game.Paused || a.world.State == game.Combat {
		pause = pause || a.key(ebiten.KeySpace)
	}
	input := game.Input{UseMouse: a.mouseMode, PaddleX: x, PaddleY: y, Left: left, Right: right, Up: up, Down: down, Launch: launch, Fire: fire, Pause: pause}
	if a.options.Mobile {
		// Idle touch input must release the virtual mouse baseline. Otherwise
		// reverse/sensitivity bonuses could jump the paddle on the next gesture.
		input.UseMouse = a.touchFrame.Move
		input.PaddleX, input.PaddleY = a.touchFrame.X, a.touchFrame.Y
		input.Launch = a.key(ebiten.KeyF2) || a.key(ebiten.KeyEnter) || a.touchFrame.Launch || gamepadLaunch
		input.Fire = ebiten.IsKeyPressed(ebiten.KeyF2) || a.touchFrame.Fire || gamepadFire
		if a.world.State == game.Paused && a.touchFrame.Launch {
			input.Launch = false
			input.Pause = true
		}
	}
	return input
}

// Draw renders original bitmaps into the native window or Android touch frame.
func (a *App) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)
	a.layout(screen.Bounds().Dx(), screen.Bounds().Dy())
	if a.introActive() {
		a.presentation.DrawIntro(a.graphics, screen)
	} else if a.nameActive() || a.scores {
		a.presentation.DrawHall(a.graphics, screen, a.steps)
	} else if a.interludeActive() {
		a.presentation.DrawInterlude(a.graphics, screen)
	} else if a.credits {
		a.presentation.DrawCredits(a.graphics, screen)
	} else if a.editor {
		a.drawEditor(screen)
	} else if a.world.State == game.Title {
		a.presentation.DrawMenu(a.graphics, screen, a.steps, a.menuScores)
		markerY := 72.0
		if a.menuScores {
			markerY = 124
		}
		a.graphics.Image(screen, "sprites/sprite-00.png", 104, markerY)
		a.graphics.CenteredText(screen, fmt.Sprintf("ROUND %02d", a.selectedRound+1), 102)
		if a.options.Mobile {
			a.graphics.CenteredText(screen, "EDITOR       HELP", 151)
		} else {
			a.graphics.CenteredText(screen, "E EDITOR H HELP C CREDITS", 151)
		}
	} else if a.world.State == game.GameOver || a.world.State == game.Won {
		a.presentation.DrawHall(a.graphics, screen, a.steps)
		if a.world.State == game.Won {
			a.graphics.CenteredText(screen, "CAMPAIGN COMPLETE", 158)
		} else {
			a.graphics.CenteredText(screen, "GAME OVER", 158)
		}
		a.graphics.CenteredText(screen, fmt.Sprintf("SCORE %06d", a.world.Score), 172)
		if a.options.Mobile {
			a.graphics.CenteredText(screen, "TAP PLAY TO RESTART", 188)
		} else {
			a.graphics.CenteredText(screen, "SPACE TO RESTART", 188)
		}
	} else {
		a.graphics.ReadyBanner = a.levelBanner || a.world.State == game.Dying
		if a.world.Combat != nil {
			a.graphics.Combat(screen, a.world, a.highScore)
		} else {
			a.graphics.Board(screen, a.world, a.highScore)
		}
		if a.world.State == game.Dying {
			a.presentation.DrawPaddleDeath(a.graphics, screen, a.world)
		}
		switch a.world.State {
		case game.Ready:
			if a.levelBanner {
				a.presentation.DrawLevelBanner(a.graphics, screen, a.world.LevelIndex)
				break
			}
			a.graphics.CenteredText(screen, fmt.Sprintf("ROUND %02d", a.world.LevelIndex+1), 145)
			if a.options.Mobile {
				a.graphics.CenteredText(screen, "FIRE TO LAUNCH", 158)
			} else {
				a.graphics.CenteredText(screen, "CLICK TO LAUNCH", 158)
			}
		case game.Paused:
			a.graphics.CenteredText(screen, "PAUSED", 145)
			if a.options.Mobile {
				a.graphics.CenteredText(screen, "TAP RESUME", 160)
			} else {
				a.graphics.CenteredText(screen, "P TO RESUME", 160)
			}
		case game.LevelClear:
			a.graphics.CenteredText(screen, "ROUND CLEAR", 145)
		}
	}
	if a.help {
		a.drawHelp(screen)
	}
	if a.messageTicks > 0 {
		a.graphics.CenteredText(screen, a.message, 178)
	}
	if a.options.Mobile {
		a.drawTouchControls(screen)
	}
	if a.capturePending && !a.captured {
		a.capture(screen)
	}
	if a.recorder != nil && a.recorder.Due() {
		screen.ReadPixels(a.moviePixels)
		if err := a.recorder.WriteFrame(a.moviePixels); err != nil {
			a.captureError = err
		} else {
			a.movieDrawnStep = a.steps
		}
	}
}

func (a *App) drawHelp(screen *ebiten.Image) {
	a.graphics.Image(screen, "images/menu.png", 0, 0)
	a.graphics.Clear(screen, 0, 54, 320, 146)
	lines := []string{"KRYPTON EGG", "MOUSE OR ARROWS MOVE", "CLICK OR F2 LAUNCH", "HOLD CLICK TO FIRE", "SPACE OR P PAUSE", "F1 PAL NTSC  F3 END", "F FULLSCREEN  M SOUND", "H CLOSE HELP"}
	if a.options.Mobile {
		lines = []string{"KRYPTON EGG", "DRAG TO MOVE PADDLE", "FIRE TO RELEASE BALL", "HOLD FIRE TO SHOOT", "PAUSE FREEZES GAME", "SOUND TOGGLE AUDIO", "MENU RETURNS TO TITLE", "TAP TO CLOSE HELP"}
	}
	for i, line := range lines {
		a.graphics.CenteredText(screen, line, float64(56+i*16))
	}
}

// Layout preserves the original aspect ratio inside a resizable high-resolution window.
func (a *App) Layout(width, height int) (int, int) {
	if a.options.Movie != "" {
		a.layout(1280, 800)
		return 1280, 800
	}
	a.layout(max(width, 1), max(height, 1))
	return max(width, 1), max(height, 1)
}

func (a *App) capture(screen *ebiten.Image) {
	if err := os.MkdirAll(filepath.Dir(a.options.Capture), 0755); err != nil {
		a.captureError = err
		return
	}
	f, err := os.Create(a.options.Capture)
	if err != nil {
		a.captureError = err
		return
	}
	err = png.Encode(f, screen)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	a.captureError, a.captured = err, true
}

func (a *App) resetWorld() {
	previous := a.world
	a.world = game.New(a.campaign, a.options.Seed)
	if previous != nil {
		a.world.PreserveRuntimeFrom(previous)
	}
	a.enforceAutomaticMode()
	a.world.SetCombatBoundary(a.combatBoundary)
	a.selectedRound = min(a.selectedRound, len(a.campaign)-1)
	a.runScoreRecorded = false
	if a.presentation != nil {
		a.highScore = a.presentation.Table.Best()
	}
	if a.pilot != nil {
		a.pilot = a.newPilot()
	}
}

func (a *App) returnToTitle() {
	a.resetWorld()
	a.editor, a.testing, a.help, a.scores, a.helpResume, a.menuScores = false, false, false, false, false, false
	a.credits, a.runScoreRecorded = false, false
	a.pendingLevelStart = nil
	a.levelBanner, a.levelBannerTicks = false, 0
	if a.presentation != nil {
		a.presentation.Name.Active = false
		a.presentation.Interlude.Active = false
	}
	a.audio.SetPaused(false)
}

func (a *App) saveScore() {
	if a.presentation == nil {
		return
	}
	if err := a.presentation.Table.Save(a.options.DataDir); err != nil {
		a.message, a.messageTicks = "SCORE SAVE FAILED", 180
	}
}

// Close releases live audio and finalizes only confirmed named score entries.
func (a *App) Close() {
	a.saveScore()
	if a.recorder != nil {
		if err := a.recorder.Close(); err != nil {
			a.captureError = err
			log.Printf("presentation recording failed: %v", err)
		}
	}
	if a.audio != nil {
		a.audio.Close()
	}
}

// CheckReport summarizes the automatic run without requiring an external emulator.
func (a *App) CheckReport() string {
	return fmt.Sprintf("smoke passed: %d updates, round %d, score %d, lives %d, %d bricks remaining", a.steps, a.world.LevelIndex+1, a.world.Score, a.world.Lives, a.world.RemainingBricks())
}

// ExportError reports capture or encoder finalization failures to the command line.
func (a *App) ExportError() error { return a.captureError }
