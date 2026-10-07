// Package game implements the deterministic rules of the Krypton Egg port.
// It has no renderer or audio dependencies, so replays and physics can run headless.
package game

import (
	"fmt"
	"math"
)

const (
	Width             = 320
	Height            = 200
	FieldLeft         = 16.0
	FieldRight        = 304.0
	FieldTop          = 17.0
	FieldBottom       = 200.0
	TicksPerSecond    = 50
	NormalPaddleWidth = 36.0
	NormalBallSpeed   = 2.23606797749979
	NormalBallRadius  = 2.5
	InitialLives      = 5
)

// State identifies the current screen or stage of a game.
type State int

const (
	Title State = iota
	Ready
	Playing
	Paused
	LevelClear
	GameOver
	Won
	Combat
)

// EffectKind identifies a bonus carried by a brick or falling drop.
type EffectKind int

const (
	NoEffect EffectKind = iota
	Grow
	Shrink
	Slow
	Fast
	Multiball
	LargeBall
	Sticky
	Laser
	Rocket
	Reverse
	Darkness
	ExtraLife
	Flying
	ScoreBonus
	MouseSensitivity
	Autopilot
	GhostPaddle
	Shield
	SuperBall
	GhostBall
	SecondPaddle
	EnhancedCannon
	RandomBonus
	WeaponMode1
	WeaponMode2
	WeaponMode4
	WeaponMode8
)

// Input uses world coordinates; keyboard movement is applied when UseMouse is false.
// Launch and Pause are edge-triggered by the caller, while Fire may be held down.
type Input struct {
	PaddleX, PaddleY      float64
	UseMouse              bool
	Left, Right, Up, Down bool
	Launch, Fire, Pause   bool
}

// Brick retains the original table code and its recovered sprite index.
type Brick struct {
	X, Y, W, H     float64
	Kind, HP       int
	Score          int
	Code           uint16
	Bonus          EffectKind
	BonusPower     int
	Destructible   bool
	Destroyed      bool
	OriginalKind   int
	TemporaryTimer int
	HitCount       int
	FlashTimer     int
}

// Level contains only geometry read from the original campaign or supplied by a caller.
type Level struct {
	Name         string
	Bricks       []Brick
	Number       int
	EnemyChoices []uint16
}

type Paddle struct {
	X, Y, W, H float64
	Slot       int
	TargetSlot int
}

type Ball struct {
	ID                   int
	X, Y, VX, VY, Radius float64
	Attached             bool
	AttachedOffset       float64
}

type Portal struct {
	X, Y float64
	Kind int
}

type Drop struct {
	X, Y, VY float64
	Kind     EffectKind
	Code     uint8
	Power    int
	Tile     int
}

type Shot struct {
	X, Y, VY float64
	Rocket   bool
	Cannon   bool
	Width    float64
	Sprite   int
}

// Enemy uses original sprite top-left coordinates and retains its behavior flags.
type Enemy struct {
	X, Y, VX, VY float64
	Kind, HP     int
	W, H         float64
	Code         uint16
	Age, Frame   int
	Destroyed    bool
}

type Effect struct {
	Kind      EffectKind
	Remaining int
	Power     int
}

type EventKind int

const (
	Bounce EventKind = iota
	BrickHit
	BrickBreak
	BallLost
	BonusSpawn
	BonusCaught
	LifeLost
	LevelStarted
	LevelCompleted
	ShotFired
	EnemySpawned
	EnemyHit
	CombatStarted
	CombatHit
	CombatCompleted
)

// Events are emitted for one tick and consumed by presentation and audio code.
type Event struct {
	Kind   EventKind
	X, Y   float64
	Value  int
	Effect EffectKind
}

// World owns all mutable simulation state. Advance it exactly once per fixed update.
type World struct {
	State                  State
	LevelIndex             int
	Score, Lives           int
	ScoreMultiplier        int
	TickCount              uint64
	Paddle                 Paddle
	SecondPaddle           *Paddle
	ShieldCharges          int
	WeaponMode             uint8
	Bricks                 []Brick
	Balls                  []Ball
	Drops                  []Drop
	Shots                  []Shot
	Enemies                []Enemy
	DoorFrame              int
	Combat                 *CombatData
	Portals                []Portal
	Effects                []Effect
	Events                 []Event
	Levels                 []Level
	seed, initialSeed      uint64
	nextBallID             int
	stateTicks             int
	resumeState            State
	enemyCooldown          int
	combatBoundary         []int
	combatShotCount        int
	combatAimVariant       bool
	lastMouseX, lastMouseY float64
	mouseSeen              bool
	previousPaddleLeft     float64
}

// New copies the supplied campaign so physics cannot modify the original level definitions.
func New(levels []Level, seed uint64) *World {
	if seed == 0 {
		seed = 1
	}
	w := &World{State: Title, Lives: InitialLives, seed: seed, initialSeed: seed}
	w.Levels = make([]Level, len(levels))
	for i, level := range levels {
		w.Levels[i] = level
		w.Levels[i].Bricks = append([]Brick(nil), level.Bricks...)
		w.Levels[i].EnemyChoices = append([]uint16(nil), level.EnemyChoices...)
	}
	w.resetPaddle()
	if len(levels) > 0 {
		w.loadLevel(0)
	}
	w.State = Title
	w.Events = nil
	return w
}

// Restart starts the recovered campaign from its first level.
func (w *World) Restart() {
	w.Score, w.Lives, w.LevelIndex, w.TickCount, w.ScoreMultiplier = 0, InitialLives, 0, 0, 0
	w.seed, w.nextBallID = w.initialSeed, 0
	w.Events = nil
	if len(w.Levels) == 0 {
		w.State = Won
		return
	}
	w.loadLevel(0)
}

func (w *World) resetPaddle() {
	w.Paddle = Paddle{X: (FieldLeft + FieldRight) / 2, Y: 189, W: NormalPaddleWidth, H: 8, Slot: 13, TargetSlot: 13}
	w.SecondPaddle, w.ShieldCharges, w.WeaponMode = nil, 0, 0
	w.mouseSeen = false
	w.previousPaddleLeft = w.Paddle.X - w.Paddle.W/2
}

// StartAt starts an actual campaign round without changing the recovered level list.
func (w *World) StartAt(index int) error {
	if index < 0 || index >= len(w.Levels) {
		return fmt.Errorf("round %d is outside the campaign", index+1)
	}
	w.Score, w.Lives, w.TickCount, w.ScoreMultiplier = 0, InitialLives, 0, 0
	w.seed, w.nextBallID, w.Events = w.initialSeed, 0, nil
	w.loadLevel(index)
	return nil
}

func (w *World) loadLevel(index int) {
	w.LevelIndex = index
	w.Bricks = append(w.Bricks[:0], w.Levels[index].Bricks...)
	for i := range w.Bricks {
		b := &w.Bricks[i]
		b.Destroyed = false
		b.TemporaryTimer, b.HitCount, b.FlashTimer = 0, 0, 0
		b.OriginalKind = b.Kind
		if b.W == 0 {
			b.W = 16
		}
		if b.H == 0 {
			b.H = 8
		}
		if b.HP <= 0 && b.Destructible {
			b.HP = 1
		}
	}
	w.Drops, w.Shots, w.Enemies, w.Effects = nil, nil, nil, nil
	w.Combat, w.enemyCooldown, w.DoorFrame = nil, 87, 0
	w.Portals = nil
	for _, brick := range w.Bricks {
		if brick.Kind == 0xf8 || brick.Kind == 0xf9 {
			w.Portals = append(w.Portals, Portal{X: brick.X + brick.W/2, Y: brick.Y + brick.H/2, Kind: brick.Kind})
		}
	}
	w.resetPaddle()
	w.attachBall()
	w.State, w.stateTicks = Ready, 0
	w.emit(LevelStarted, w.Paddle.X, w.Paddle.Y, index+1, NoEffect)
}

func (w *World) attachBall() {
	w.nextBallID++
	w.Balls = []Ball{{ID: w.nextBallID, X: w.Paddle.X, Y: w.Paddle.Y - w.Paddle.H/2 - NormalBallRadius, Radius: NormalBallRadius, Attached: true}}
}

// ResetPointer starts a fresh touch gesture without reusing the virtual mouse
// position from before a pause, lifecycle suspension, or reversed-controls bonus.
func (w *World) ResetPointer() { w.mouseSeen = false }

// Tick advances game rules by one original PAL update, or 1/50 second.
func (w *World) Tick(input Input) {
	w.Events = w.Events[:0]
	if input.Pause {
		if w.State == Paused {
			w.State = w.resumeState
			return
		}
		if w.State == Playing || w.State == Ready || w.State == Combat {
			w.resumeState, w.State = w.State, Paused
			return
		}
	}
	if w.State == Paused {
		return
	}
	w.TickCount++
	w.stateTicks++
	switch w.State {
	case Title, GameOver, Won:
		if input.Launch {
			w.Restart()
		}
		return
	case LevelClear:
		if input.Launch || w.stateTicks >= 120 {
			if (w.LevelIndex+1)%10 == 0 {
				w.startCombat()
				return
			}
			if w.LevelIndex+1 >= len(w.Levels) {
				w.State = Won
			} else {
				w.loadLevel(w.LevelIndex + 1)
			}
		}
		return
	case Combat:
		w.updateCombat(input)
		return
	}
	w.previousPaddleLeft = w.Paddle.X - w.Paddle.W/2
	w.movePaddle(input)
	w.updatePaddleSize()
	for i := range w.Balls {
		if w.Balls[i].Attached {
			w.Balls[i].X, w.Balls[i].Y = w.Paddle.X+w.Balls[i].AttachedOffset, w.Paddle.Y-w.Paddle.H/2-w.Balls[i].Radius-0.05
			if input.Launch {
				w.Balls[i].Attached = false
				w.Balls[i].VX, w.Balls[i].VY = 1, -2
			}
		}
	}
	if w.State == Ready {
		if input.Launch {
			w.State, w.stateTicks = Playing, 0
		} else {
			return
		}
	}
	w.updateEffects()
	w.updateBricks()
	if input.Fire || input.Launch {
		if w.Active(Laser) || w.Active(Rocket) || w.Active(EnhancedCannon) || w.WeaponMode != 0 {
			w.fire()
		}
	}
	w.updateShots()
	w.updateEnemies()
	w.updateBalls()
	w.updateDrops()
	if w.remainingBricks() == 0 {
		w.State, w.stateTicks = LevelClear, 0
		w.Drops, w.Shots = nil, nil
		w.emit(LevelCompleted, w.Paddle.X, w.Paddle.Y, w.LevelIndex+1, NoEffect)
		return
	}
	if len(w.Balls) == 0 {
		w.Lives--
		w.emit(LifeLost, w.Paddle.X, w.Paddle.Y, w.Lives, NoEffect)
		if w.Lives <= 0 {
			w.State, w.stateTicks = GameOver, 0
			return
		}
		w.Effects, w.Drops, w.Shots = nil, nil, nil
		w.Enemies, w.enemyCooldown = nil, 87
		w.resetPaddle()
		w.attachBall()
		w.State, w.stateTicks = Ready, 0
	}
}

func (w *World) movePaddle(in Input) {
	x, y := w.Paddle.X, w.Paddle.Y
	if in.UseMouse {
		x, y = in.PaddleX, in.PaddleY
		if (w.Active(MouseSensitivity) || w.Active(Reverse)) && w.mouseSeen {
			deltaX, deltaY := in.PaddleX-w.lastMouseX, in.PaddleY-w.lastMouseY
			if w.Active(Reverse) {
				deltaX = -deltaX
			}
			factor := 1.0
			if w.Active(MouseSensitivity) {
				factor = 3
			}
			x, y = w.Paddle.X+deltaX*factor, w.Paddle.Y+deltaY*factor
		}
		w.lastMouseX, w.lastMouseY, w.mouseSeen = in.PaddleX, in.PaddleY, true
	} else {
		w.mouseSeen = false
		direction := 1.0
		if w.Active(Reverse) {
			direction = -1
		}
		if in.Left {
			x -= 3.5 * direction
		}
		if in.Right {
			x += 3.5 * direction
		}
		if in.Up {
			y -= 2.5
		}
		if in.Down {
			y += 2.5
		}
	}
	if w.Active(Autopilot) && len(w.Balls) > 0 {
		lowest := w.Balls[0]
		for _, ball := range w.Balls {
			if ball.Y > lowest.Y {
				lowest = ball
			}
		}
		x = lowest.X
	}
	w.Paddle.X = clamp(x, FieldLeft+w.Paddle.W/2, FieldRight-1-w.Paddle.W/2)
	// The native paddle moves vertically only after catching bonus 14.
	if w.Active(Flying) {
		w.Paddle.Y = clamp(y, 32+w.Paddle.H/2, 185+w.Paddle.H/2)
	} else {
		w.Paddle.Y = 185 + w.Paddle.H/2
	}
}

func (w *World) emit(kind EventKind, x, y float64, value int, effect EffectKind) {
	w.Events = append(w.Events, Event{Kind: kind, X: x, Y: y, Value: value, Effect: effect})
}

func (w *World) remainingBricks() int {
	n := 0
	for _, b := range w.Bricks {
		if b.Destructible && !b.Destroyed {
			n++
		}
	}
	return n
}

// RemainingBricks returns the number of breakable bricks still on the board.
func (w *World) RemainingBricks() int { return w.remainingBricks() }

// Active reports whether a timed bonus is currently active.
func (w *World) Active(kind EffectKind) bool {
	for _, e := range w.Effects {
		if e.Kind == kind && e.Remaining != 0 {
			return true
		}
	}
	return false
}

// ApplyBonus exposes the same bonus path used by caught drops for replays and tests.
func (w *World) ApplyBonus(kind EffectKind) {
	w.ApplyBonusPower(kind, 0)
}

// ApplyBonusPower preserves the two-bit strength embedded in the original table.
func (w *World) ApplyBonusPower(kind EffectKind, power int) {
	power = max(0, min(3, power))
	if kind == NoEffect {
		return
	}
	if kind == Grow || kind == Shrink {
		if kind == Grow {
			w.resizePaddle(2 * (power + 1))
		} else {
			w.resizePaddle(-(2*power + 1))
		}
	}
	if kind == RandomBonus {
		for {
			code := uint8(1 + int(w.random()*27))
			if code != 12 && code != 22 {
				w.ApplyBonusPower(BonusFromCode(code<<2), power)
				return
			}
		}
	}
	if kind == Shield {
		w.ShieldCharges += power + 1
	}
	if kind == SecondPaddle {
		paddle := w.Paddle
		w.SecondPaddle = &paddle
	}
	if kind == WeaponMode1 {
		w.WeaponMode |= 1
	}
	if kind == WeaponMode2 {
		w.WeaponMode |= 2
	}
	if kind == WeaponMode4 {
		w.WeaponMode |= 4
	}
	if kind == WeaponMode8 {
		w.WeaponMode |= 8
	}
	if kind == ExtraLife {
		w.Lives += power + 1
		return
	}
	if kind == ScoreBonus {
		w.ScoreMultiplier = int(uint16(w.ScoreMultiplier + power + 1))
		return
	}
	if kind == Multiball {
		original := append([]Ball(nil), w.Balls...)
		for _, source := range original {
			if source.Attached {
				continue
			}
			if len(w.Balls) >= 8 {
				return
			}
			angle := (w.random()*0.6 + 0.2)
			if w.random() < 0.5 {
				angle = -angle
			}
			w.nextBallID++
			ball := source
			ball.ID = w.nextBallID
			ball.VX = source.VX*math.Cos(angle) - source.VY*math.Sin(angle)
			ball.VY = source.VX*math.Sin(angle) + source.VY*math.Cos(angle)
			w.Balls = append(w.Balls, ball)
			return
		}
		return
	}
	if kind == Fast || kind == Slow {
		for i := range w.Balls {
			ball := &w.Balls[i]
			if ball.Attached {
				continue
			}
			change := float64(power + 1)
			if kind == Fast {
				ball.VX = math.Copysign(math.Min(4, math.Abs(ball.VX)+change), ball.VX)
				ball.VY = math.Copysign(math.Min(4, math.Abs(ball.VY)+change), ball.VY)
			}
			if kind == Slow {
				ball.VX = math.Copysign(math.Max(1, math.Abs(ball.VX)-change), ball.VX)
				ball.VY = math.Copysign(math.Max(1, math.Abs(ball.VY)-change), ball.VY)
			}
		}
	}
	if kind == LargeBall {
		for i := range w.Balls {
			for step := 0; step <= power; step++ {
				diameter := int(w.Balls[i].Radius*2 + 0.5)
				switch diameter {
				case 5:
					diameter = 6
				case 6, 8:
					diameter += 2
				case 10, 13:
					diameter += 3
				default:
					diameter = 16
				}
				w.Balls[i].Radius = float64(min(16, diameter)) / 2
			}
		}
	}
	for _, opposite := range [][2]EffectKind{{Grow, Shrink}, {Slow, Fast}, {Laser, Rocket}} {
		if kind == opposite[0] {
			w.removeEffect(opposite[1])
		}
		if kind == opposite[1] {
			w.removeEffect(opposite[0])
		}
	}
	duration := -1
	switch kind {
	case Darkness:
		duration = 64 * (power + 1)
	case Autopilot:
		duration = 512 * (power + 1)
	case GhostPaddle:
		duration = 32 * (power + 1)
	case GhostBall:
		duration = 256 * (power + 1)
	}
	for i := range w.Effects {
		if w.Effects[i].Kind == kind {
			w.Effects[i].Remaining = duration
			w.Effects[i].Power = power
			w.syncEffects()
			return
		}
	}
	w.Effects = append(w.Effects, Effect{Kind: kind, Remaining: duration, Power: power})
	w.syncEffects()
}

func (w *World) removeEffect(kind EffectKind) {
	for i := 0; i < len(w.Effects); {
		if w.Effects[i].Kind == kind {
			w.Effects = append(w.Effects[:i], w.Effects[i+1:]...)
		} else {
			i++
		}
	}
}

func (w *World) updateEffects() {
	changed := false
	for i := 0; i < len(w.Effects); {
		if w.Effects[i].Remaining > 0 {
			if w.Effects[i].Kind == GhostBall {
				w.Effects[i].Remaining -= max(1, len(w.Balls))
				if w.Effects[i].Remaining <= 0 {
					w.Effects[i].Remaining = 1
					for _, ball := range w.Balls {
						if ball.Y > 152 {
							w.Effects[i].Remaining = 0
							break
						}
					}
				}
			} else {
				w.Effects[i].Remaining--
			}
		}
		if w.Effects[i].Remaining == 0 {
			w.Effects = append(w.Effects[:i], w.Effects[i+1:]...)
			changed = true
		} else {
			i++
		}
	}
	if changed {
		w.syncEffects()
	}
}

func (w *World) syncEffects() {
	w.Paddle.X = clamp(w.Paddle.X, FieldLeft+w.Paddle.W/2, FieldRight-1-w.Paddle.W/2)
}

// PaddleWidths is the original collision-width table for slots 6 through 24.
var PaddleWidths = [...]float64{10, 14, 17, 20, 24, 28, 32, 36, 38, 42, 46, 50, 54, 58, 62, 66, 70, 74, 78}

func (w *World) resizePaddle(steps int) {
	if w.Paddle.Slot < 7 || w.Paddle.Slot > 24 {
		w.Paddle.Slot = 13
	}
	w.Paddle.TargetSlot = max(7, min(24, w.Paddle.Slot+steps))
}

// The original expansion animation advances one recovered width slot per frame.
func (w *World) updatePaddleSize() {
	if w.Paddle.TargetSlot < 7 || w.Paddle.TargetSlot > 24 {
		w.Paddle.TargetSlot = w.Paddle.Slot
	}
	if w.Paddle.Slot < w.Paddle.TargetSlot {
		w.Paddle.Slot++
	}
	if w.Paddle.Slot > w.Paddle.TargetSlot {
		w.Paddle.Slot--
	}
	w.Paddle.W = PaddleWidths[w.Paddle.Slot-6]
	w.syncEffects()
}

// EffectPower returns the original strength of an active effect.
func (w *World) EffectPower(kind EffectKind) int {
	for _, effect := range w.Effects {
		if effect.Kind == kind && effect.Remaining != 0 {
			return effect.Power
		}
	}
	return 0
}

func (w *World) random() float64 {
	w.seed ^= w.seed << 13
	w.seed ^= w.seed >> 7
	w.seed ^= w.seed << 17
	return float64(w.seed>>11) / float64(uint64(1)<<53)
}

func clamp(value, low, high float64) float64 { return math.Max(low, math.Min(value, high)) }
