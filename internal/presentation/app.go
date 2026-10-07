package presentation

import (
	"encoding/json"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"krytonegg/assets"
	"krytonegg/internal/game"
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
	visuals                         []visualEffect
	touchController                 touch.Controller
	touchSamples                    []touch.Sample
	touchFrame                      touch.Frame
	touchErase                      bool
}

type visualEffect struct {
	enemy bool
	x, y  float64
	age   int
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
	audio, err := sound.New(options.Muted)
	if err != nil {
		return nil, err
	}
	a := &App{world: game.New(levels, options.Seed), campaign: levels, combatBoundary: combatMetadata.Boundary, graphics: graphics, audio: audio, table: table, options: options, mouseMode: true, highScore: 5000}
	a.world.SetCombatBoundary(a.combatBoundary)
	a.readScore()
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
	a.layout(game.Width*options.Scale, game.Height*options.Scale)
	return a, nil
}

func just(key ebiten.Key) bool { return inpututil.IsKeyJustPressed(key) }

func (a *App) key(key ebiten.Key) bool { return a.options.SmokeTicks == 0 && just(key) }

// Update is the sole owner of simulation advances and input edges.
func (a *App) Update() error {
	if a.captureError != nil {
		return a.captureError
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
	if a.options.Mobile && a.options.SmokeTicks == 0 {
		a.sampleTouches()
		if a.touchActions() {
			return nil
		}
	}
	if a.key(ebiten.KeyF11) || a.key(ebiten.KeyF) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	if a.key(ebiten.KeyM) {
		a.audio.ToggleMute()
	}
	if a.key(ebiten.KeyF1) {
		if ebiten.TPS() == 50 {
			ebiten.SetTPS(60)
		} else {
			ebiten.SetTPS(50)
		}
	}
	if a.key(ebiten.KeyH) {
		a.help = !a.help
		if a.help && (a.world.State == game.Playing || a.world.State == game.Ready || a.world.State == game.Combat) {
			a.world.Tick(game.Input{Pause: true})
			a.helpResume = true
		} else if !a.help && a.helpResume {
			a.world.Tick(game.Input{Pause: true})
			a.helpResume = false
		}
	}
	if a.key(ebiten.KeyF3) {
		a.saveScore()
		a.returnToTitle()
	}
	if a.key(ebiten.KeyQ) && (a.world.State == game.Title || a.world.State == game.Paused) {
		return ebiten.Termination
	}
	if a.help {
		a.audio.SetPaused(true)
		return nil
	}
	if a.key(ebiten.KeyEscape) {
		if a.scores {
			a.scores = false
			return nil
		}
		if a.testing {
			a.testing, a.editor = false, true
			a.resetWorld()
			a.audio.Music("")
			return nil
		}
		if a.editor {
			a.returnToTitle()
			return nil
		}
		if a.world.State == game.Playing || a.world.State == game.Ready || a.world.State == game.Paused || a.world.State == game.Combat {
			a.world.Tick(game.Input{Pause: true})
			a.audio.SetPaused(a.world.State == game.Paused)
			return nil
		}
		a.returnToTitle()
	}
	if a.scores {
		a.audio.SetPaused(false)
		a.audio.Music("halloffame")
		if a.key(ebiten.KeySpace) || a.key(ebiten.KeyEnter) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			a.scores = false
			a.menuScores = false
		}
		return nil
	}
	if a.editor {
		if a.options.SmokeTicks > 0 {
			return nil
		}
		return a.updateEditor()
	}
	if a.world.State == game.Title {
		if a.key(ebiten.KeyE) {
			a.openEditor()
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
	input := a.input()
	if a.options.SmokeTicks > 0 {
		input = game.Input{}
		input.UseMouse = true
		input.PaddleX, input.PaddleY = a.world.Paddle.X, 190
		if len(a.world.Balls) > 0 {
			input.PaddleX = a.world.Balls[0].X
		}
		input.Launch = a.steps > 1 && (a.world.State == game.Title || a.world.State == game.Ready || a.world.State == game.LevelClear)
		input.Fire = true
		if a.world.Combat != nil {
			input.PaddleY = 70
		}
	}
	if a.world.State == game.Title && input.Launch {
		selectedScores := a.menuScores
		if a.options.SmokeTicks == 0 && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := a.graphics.WorldPosition(a.mouseX, a.mouseY)
			selectedScores = x >= 96 && x <= 224 && y >= 116 && y <= 145
		}
		if selectedScores {
			a.scores = true
			return nil
		}
		if err := a.world.StartAt(a.selectedRound); err != nil {
			return err
		}
	} else {
		a.world.Tick(input)
	}
	if a.world.Score > a.highScore {
		a.highScore = a.world.Score
	}
	for _, event := range a.world.Events {
		a.audio.Event(event)
		if event.Kind == game.EnemyHit {
			a.visuals = append(a.visuals, visualEffect{enemy: true, x: event.X - 8, y: event.Y - 8})
		}
		if event.Kind == game.LifeLost {
			a.visuals = append(a.visuals, visualEffect{x: event.X - 18, y: event.Y - 4})
		}
		if event.Kind == game.LifeLost || event.Kind == game.LevelCompleted {
			a.saveScore()
		}
	}
	if a.world.State != game.Paused {
		live := a.visuals[:0]
		for _, effect := range a.visuals {
			effect.age++
			limit := 28
			if effect.enemy {
				limit = 42
			}
			if effect.age < limit {
				live = append(live, effect)
			}
		}
		a.visuals = live
	}
	if a.world.State == game.Title {
		a.audio.Music("intro")
	} else if a.world.State == game.GameOver || a.world.State == game.Won {
		a.audio.Music("halloffame")
	} else {
		a.audio.Music("")
	}
	a.audio.SetPaused(a.world.State == game.Paused)
	return nil
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
	if a.editor {
		a.drawEditor(screen)
	} else if a.scores {
		a.graphics.Image(screen, "images/fame.png", 0, 0)
		a.graphics.CenteredText(screen, "HIGH SCORE", 112)
		a.graphics.CenteredText(screen, fmt.Sprintf("%06d", a.highScore), 136)
		a.graphics.CenteredText(screen, "CLICK TO RETURN", 176)
	} else if a.world.State == game.Title {
		a.graphics.Image(screen, "images/menu.png", 0, 0)
		a.graphics.Clear(screen, 120, 72, 80, 16)
		a.graphics.Clear(screen, 112, 124, 96, 16)
		a.graphics.CenteredText(screen, "PLAY", 72)
		a.graphics.CenteredText(screen, "SCORES", 124)
		markerY := 72.0
		if a.menuScores {
			markerY = 124
		}
		a.graphics.Image(screen, "sprites/sprite-00.png", 104, markerY)
		a.graphics.CenteredText(screen, fmt.Sprintf("ROUND %02d", a.selectedRound+1), 102)
		if a.options.Mobile {
			a.graphics.CenteredText(screen, "EDITOR       HELP", 151)
		} else {
			a.graphics.CenteredText(screen, "E CONSTRUCTION  H HELP", 151)
		}
	} else if a.world.State == game.GameOver || a.world.State == game.Won {
		a.graphics.Image(screen, "images/fame.png", 0, 0)
		if a.world.State == game.Won {
			a.graphics.CenteredText(screen, "CAMPAIGN COMPLETE", 136)
		} else {
			a.graphics.CenteredText(screen, "GAME OVER", 136)
		}
		a.graphics.CenteredText(screen, fmt.Sprintf("SCORE %06d", a.world.Score), 152)
		if a.options.Mobile {
			a.graphics.CenteredText(screen, "TAP PLAY TO RESTART", 176)
		} else {
			a.graphics.CenteredText(screen, "SPACE TO RESTART", 176)
		}
	} else {
		if a.world.Combat != nil {
			a.graphics.Combat(screen, a.world, a.highScore)
		} else {
			a.graphics.Board(screen, a.world, a.highScore)
		}
		for _, effect := range a.visuals {
			if effect.enemy {
				a.graphics.Image(screen, fmt.Sprintf("sprites/enemy-death-%d.png", effect.age/2), effect.x, effect.y)
			} else {
				a.graphics.Sprite(screen, 48+effect.age/4, effect.x, effect.y)
			}
		}
		switch a.world.State {
		case game.Ready:
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
}

func (a *App) drawHelp(screen *ebiten.Image) {
	a.graphics.Image(screen, "images/menu.png", 0, 0)
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

func (a *App) readScore() {
	data, err := os.ReadFile(filepath.Join(a.options.DataDir, "highscore.json"))
	if err != nil {
		return
	}
	var stored struct {
		Score int `json:"score"`
	}
	if json.Unmarshal(data, &stored) == nil {
		a.highScore = max(5000, stored.Score)
	}
}

func (a *App) resetWorld() {
	a.world = game.New(a.campaign, a.options.Seed)
	a.world.SetCombatBoundary(a.combatBoundary)
	a.selectedRound = min(a.selectedRound, len(a.campaign)-1)
	a.visuals = nil
}

func (a *App) returnToTitle() {
	a.resetWorld()
	a.editor, a.testing, a.help, a.scores, a.helpResume, a.menuScores = false, false, false, false, false, false
	a.audio.SetPaused(false)
}

func (a *App) saveScore() {
	data, _ := json.MarshalIndent(struct {
		Score int `json:"score"`
	}{a.highScore}, "", "  ")
	if err := atomicWrite(filepath.Join(a.options.DataDir, "highscore.json"), append(data, '\n')); err != nil {
		a.message, a.messageTicks = "SCORE SAVE FAILED", 180
	}
}

// Close releases audio resources and persists the local high score.
func (a *App) Close() {
	if a.highScore > 0 {
		a.saveScore()
	}
	a.audio.Close()
}

// CheckReport summarizes the automatic run without requiring an external emulator.
func (a *App) CheckReport() string {
	return fmt.Sprintf("smoke passed: %d updates, round %d, score %d, lives %d, %d bricks remaining", a.steps, a.world.LevelIndex+1, a.world.Score, a.world.Lives, a.world.RemainingBricks())
}
