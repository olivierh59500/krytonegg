package game

import (
	"encoding/binary"
	"math"
	"os"
	"reflect"
	"testing"
)

func testWorld() *World {
	return New([]Level{{Bricks: []Brick{{X: 120, Y: 40, W: 16, H: 8, Kind: 1, HP: 1, Score: 10, Destructible: true}}}}, 7)
}

func startWorld(w *World) {
	w.Tick(Input{Launch: true})
	w.Tick(Input{Launch: true})
}

func TestOriginalTableCoordinatesAndCodes(t *testing.T) {
	data := make([]byte, LevelByteSize*2)
	binary.BigEndian.PutUint16(data[(4*LevelColumns+3)*2:], 0x5a31)
	binary.BigEndian.PutUint16(data[LevelByteSize+(2*LevelColumns+17)*2:], 0x0062)
	levels, err := LoadLevels(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 2 || len(levels[0].Bricks) != 1 {
		t.Fatalf("unexpected table shape: %#v", levels)
	}
	brick := levels[0].Bricks[0]
	if brick.X != 64 || brick.Y != 56 || brick.Kind != 0x31 || brick.Code != 0x5a31 || !brick.Destructible {
		t.Fatalf("original code or geometry changed: %#v", brick)
	}
	if levels[1].Bricks[0].Destructible || levels[1].Bricks[0].X != 288 {
		t.Fatalf("permanent brick decoded incorrectly: %#v", levels[1].Bricks[0])
	}
	if _, err := LoadLevels(data[:len(data)-1]); err == nil {
		t.Fatal("truncated level accepted")
	}
}

func TestFastBallCannotTunnelThroughBrick(t *testing.T) {
	w := testWorld()
	startWorld(w)
	w.Balls = []Ball{{ID: 1, X: 128, Y: 25, VY: 20, Radius: 3}}
	w.Tick(Input{})
	if !w.Bricks[0].Destroyed || w.Score != 10 || w.State != LevelClear {
		t.Fatalf("fast ball missed brick: %+v", w)
	}
	if w.Balls[0].VY >= 0 {
		t.Fatalf("ball did not reflect: %+v", w.Balls[0])
	}
}

func TestCornerContactPreservesSpeed(t *testing.T) {
	w := New([]Level{{Bricks: []Brick{{X: 48, Y: 48, W: 16, H: 8, Kind: 1, HP: 2, Destructible: true}}}}, 1)
	startWorld(w)
	w.Balls = []Ball{{ID: 1, X: 45, Y: 45, VX: 4, VY: 4, Radius: 3}}
	w.Tick(Input{})
	ball := w.Balls[0]
	if ball.VX >= 0 || ball.VY >= 0 {
		t.Fatalf("corner normal did not reflect both components: %+v", ball)
	}
	if math.Abs(math.Hypot(ball.VX, ball.VY)-math.Sqrt(32)) > 1e-9 {
		t.Fatalf("collision changed speed: %+v", ball)
	}
	if w.Bricks[0].HP != 1 {
		t.Fatalf("one contact caused repeated damage: %d", w.Bricks[0].HP)
	}
}

func TestStationaryPaddlePreservesNativeHorizontalVelocity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		offset float64
	}{{"left", -12}, {"center", 0}, {"right", 12}} {
		t.Run(tc.name, func(t *testing.T) {
			w := testWorld()
			startWorld(w)
			w.Balls = []Ball{{ID: 1, X: w.Paddle.X + tc.offset, Y: 182, VX: 1, VY: 3, Radius: 3}}
			w.Tick(Input{})
			ball := w.Balls[0]
			if ball.VY != -3 || ball.VX != 1 {
				t.Fatalf("stationary paddle changed native trajectory: %+v", ball)
			}
		})
	}
}

func TestOnlyLastBallCostsLife(t *testing.T) {
	w := testWorld()
	startWorld(w)
	w.Balls = []Ball{{ID: 1, X: 25, Y: 204, VY: 1, Radius: 3}, {ID: 2, X: 200, Y: 100, VY: -1, Radius: 3}}
	w.Tick(Input{})
	if w.Lives != InitialLives || len(w.Balls) != 1 || w.State != Playing {
		t.Fatalf("multiball loss charged a life: %+v", w)
	}
	w.Balls[0].Y, w.Balls[0].VY = 204, 1
	w.Tick(Input{})
	if w.Lives != InitialLives-1 || w.State != Ready || len(w.Balls) != 1 || !w.Balls[0].Attached {
		t.Fatalf("last ball did not respawn: %+v", w)
	}
	w.Tick(Input{})
	if w.Lives != InitialLives-1 {
		t.Fatal("ready state charged another life")
	}
}

func TestBonusesExpireAndMultiballKeepsIndependentIDs(t *testing.T) {
	w := testWorld()
	startWorld(w)
	w.ApplyBonus(Grow)
	w.updatePaddleSize()
	w.updatePaddleSize()
	if w.Paddle.W != 42 || w.Paddle.Slot != 15 {
		t.Fatal("growth not applied")
	}
	w.ApplyBonus(Shrink)
	w.updatePaddleSize()
	if w.Active(Grow) || !w.Active(Shrink) || w.Paddle.W != 38 || w.Paddle.Slot != 14 {
		t.Fatal("opposite size effects did not replace one another")
	}
	w.ApplyBonus(Darkness)
	for i := range w.Effects {
		if w.Effects[i].Kind == Darkness {
			w.Effects[i].Remaining = 1
		}
	}
	w.Tick(Input{})
	if w.Active(Darkness) || w.Paddle.W != 38 {
		t.Fatal("palette effect expiry changed persistent paddle geometry")
	}
	w.ApplyBonus(Multiball)
	if len(w.Balls) != 2 {
		t.Fatalf("multiball created %d balls", len(w.Balls))
	}
	seen := map[int]bool{}
	for _, ball := range w.Balls {
		if seen[ball.ID] {
			t.Fatal("multiball reused an ID")
		}
		seen[ball.ID] = true
		if math.Abs(math.Hypot(ball.VX, ball.VY)-NormalBallSpeed) > 1e-9 {
			t.Fatal("multiball changed speed")
		}
	}
}

func TestPauseFreezesWorldAndCampaignEnds(t *testing.T) {
	w := testWorld()
	startWorld(w)
	w.Tick(Input{Pause: true})
	ball := w.Balls[0]
	tickCount := w.TickCount
	w.Tick(Input{Left: true, Launch: true})
	if w.State != Paused || w.Balls[0] != ball || w.TickCount != tickCount {
		t.Fatal("pause advanced gameplay")
	}
	w.Tick(Input{Pause: true})
	if w.State != Playing {
		t.Fatal("pause did not resume gameplay")
	}
	w.Bricks[0].Destroyed = true
	w.Tick(Input{})
	w.Tick(Input{Launch: true})
	if w.State != Won {
		t.Fatal("campaign did not finish after the final recovered level")
	}
}

func TestSimulationIsDeterministicAndDoesNotMutateLevels(t *testing.T) {
	a, b := testWorld(), testWorld()
	for tick := 0; tick < 900; tick++ {
		in := Input{Launch: tick == 0 || tick == 1 || tick%100 == 0, Left: tick%120 < 60, Right: tick%120 >= 60}
		a.Tick(in)
		b.Tick(in)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("replay diverged at tick %d", tick)
		}
	}
	if a.Levels[0].Bricks[0].Destroyed || a.Levels[0].Bricks[0].HP != 1 {
		t.Fatal("simulation modified campaign data")
	}
}

func TestLayeredOriginalBricksRetainBonusStrength(t *testing.T) {
	data := make([]byte, LevelByteSize)
	binary.BigEndian.PutUint16(data, 0x0743)
	levels, err := LoadLevels(data)
	if err != nil {
		t.Fatal(err)
	}
	w := New(levels, 1)
	w.damageBrick(0)
	if w.Bricks[0].Kind != 0x33 || w.Bricks[0].HP != 1 || w.Bricks[0].Destroyed || len(w.Drops) != 0 {
		t.Fatalf("layer did not peel correctly: %+v", w.Bricks[0])
	}
	w.damageBrick(0)
	if !w.Bricks[0].Destroyed || len(w.Drops) != 1 || w.Drops[0].Kind != Grow || w.Drops[0].Power != 3 || w.Drops[0].Code != 7 {
		t.Fatalf("original embedded bonus was lost: %+v", w.Drops)
	}
	if w.Score != 1 {
		t.Fatalf("original score units changed: %d", w.Score)
	}
}

func TestScoreBonusChangesMultiplierAndExtraLifeUsesStrength(t *testing.T) {
	w := testWorld()
	w.ApplyBonusPower(ScoreBonus, 2)
	if w.Score != 0 || w.ScoreMultiplier != 3 {
		t.Fatal("score bonus should increase multiplier")
	}
	w.damageBrick(0)
	if w.Score != 80 {
		t.Fatalf("brick did not use multiplier: %d", w.Score)
	}
	w.ApplyBonusPower(ExtraLife, 3)
	if w.Lives != InitialLives+4 {
		t.Fatalf("life bonus did not preserve strength: %d", w.Lives)
	}
}

func TestTemporaryBlocksRegenerateWithoutCountingForCompletion(t *testing.T) {
	w := New([]Level{{Bricks: []Brick{{X: 40, Y: 40, Kind: 0xfa, HP: -1}, {X: 80, Y: 40, Kind: 2, HP: 1, Destructible: true}}}}, 1)
	w.damageBrick(0)
	if !w.Bricks[0].Destroyed || w.Bricks[0].TemporaryTimer != 33 || w.RemainingBricks() != 1 {
		t.Fatal("temporary brick did not vanish independently of completion")
	}
	for i := 0; i < 132; i++ {
		w.TickCount++
		w.updateBricks()
	}
	if w.Bricks[0].Destroyed || w.Bricks[0].Kind != 0xfa {
		t.Fatal("temporary brick did not regenerate")
	}
}

func TestStartAtUsesActualCampaignIndex(t *testing.T) {
	w := New([]Level{{Name: "FIRST"}, {Name: "SECOND"}}, 1)
	if err := w.StartAt(1); err != nil {
		t.Fatal(err)
	}
	if w.LevelIndex != 1 || w.State != Ready || w.Lives != InitialLives || len(w.Levels) != 2 {
		t.Fatal("round selector replaced campaign")
	}
	if err := w.StartAt(2); err == nil {
		t.Fatal("out-of-range round accepted")
	}
}

func TestOriginalCombatTriggersAfterEveryTenRounds(t *testing.T) {
	levels := make([]Level, OriginalLevelCount)
	w := New(levels, 1)
	w.LevelIndex, w.State = 9, LevelClear
	w.Tick(Input{Launch: true})
	if w.State != Combat || w.Combat == nil || w.Combat.Number != 1 || w.Combat.PlayerEnergy != 448 || w.Combat.BossEnergy != 896 {
		t.Fatalf("first original combat did not start: %+v", w.Combat)
	}
	w.Combat.BossEnergy = 8
	w.Combat.PlayerShots = []CombatShot{{X: 244, Y: 68, VX: 3}}
	w.Tick(Input{})
	if w.State != Ready || w.LevelIndex != 10 {
		t.Fatal("combat victory did not resume round eleven")
	}
	w.LevelIndex, w.State = 59, LevelClear
	w.Tick(Input{Launch: true})
	if w.Combat.Number != 6 {
		t.Fatal("last combat did not preserve campaign number")
	}
	w.Combat.BossEnergy = 4
	w.Combat.PlayerShots = []CombatShot{{X: 244, Y: 91, VX: 3}}
	w.Tick(Input{})
	if w.State != Won {
		t.Fatal("campaign ended before or after the sixth boss")
	}
}

func TestCombatWeakspotAndImmediateDefeat(t *testing.T) {
	w := New(make([]Level, 10), 1)
	w.LevelIndex = 9
	w.startCombat()
	w.Combat.PlayerShots = []CombatShot{{X: 244, Y: 67, VX: 3}, {X: 244, Y: 68, VX: 3}, {X: 244, Y: 91, VX: 3}, {X: 244, Y: 92, VX: 3}}
	w.Tick(Input{})
	if w.Combat.BossEnergy != 880 {
		t.Fatalf("weakspot boundaries changed: %d", w.Combat.BossEnergy)
	}
	w.Combat.PlayerEnergy = 64
	w.Combat.BossShots = []CombatShot{{X: 35, Y: w.Combat.ShipY + 16, VX: -5}}
	w.Tick(Input{})
	if w.State != GameOver || w.Combat.PlayerEnergy != 0 || w.Lives != InitialLives {
		t.Fatal("original combat defeat must end the run independently of reserves")
	}
}

func TestCombatUsesRecoveredSilhouetteInsteadOfRectangle(t *testing.T) {
	w := New(make([]Level, 10), 1)
	w.LevelIndex = 9
	w.startCombat()
	profile := make([]int, 40)
	for i := range profile {
		profile[i] = 245
	}
	profile[20] = 230
	w.SetCombatBoundary(profile)
	w.Combat.PlayerShots = []CombatShot{{X: 228, Y: 82, VX: 3}}
	w.Tick(Input{})
	if w.Combat.BossEnergy != 888 || len(w.Combat.PlayerShots) != 0 {
		t.Fatal("decoded boss boundary was ignored")
	}
}

func TestDecodedEnemyFlagsControlPaddleHazard(t *testing.T) {
	for _, lethal := range []bool{false, true} {
		w := testWorld()
		startWorld(w)
		code := uint16(0x8000)
		if lethal {
			code |= 0x1000
		}
		w.Enemies = []Enemy{{X: 150, Y: 185, W: 16, H: 23, Kind: 2, Code: code, HP: 1}}
		w.Tick(Input{})
		if lethal && (w.Lives != InitialLives-1 || w.State != Ready) {
			t.Fatal("lethal original enemy did not cost one life")
		}
		if !lethal && (w.Lives != InitialLives || w.State != Playing || len(w.Enemies) != 0) {
			t.Fatal("normal enemy was lethal on paddle contact")
		}
	}
}

func TestVerticalMovementRequiresOriginalBonus(t *testing.T) {
	w := testWorld()
	startWorld(w)
	w.Tick(Input{UseMouse: true, PaddleX: 160, PaddleY: 60})
	if w.Paddle.Y != 189 {
		t.Fatal("native paddle should start with horizontal movement")
	}
	w.ApplyBonus(Flying)
	w.Tick(Input{UseMouse: true, PaddleX: 160, PaddleY: 60})
	if w.Paddle.Y != 60 {
		t.Fatal("bonus 14 did not enable vertical movement")
	}
}

func TestOriginalBallGrowthAndSpeedLimits(t *testing.T) {
	w := testWorld()
	startWorld(w)
	w.ApplyBonusPower(LargeBall, 3)
	if w.Balls[0].Radius != 6.5 {
		t.Fatalf("four growth steps should produce diameter13: %g", w.Balls[0].Radius*2)
	}
	w.ApplyBonusPower(LargeBall, 3)
	if w.Balls[0].Radius != 8 {
		t.Fatal("ball growth exceeded the original maximum")
	}
	w.ApplyBonusPower(Fast, 3)
	if w.Balls[0].VX != 4 || w.Balls[0].VY != -4 {
		t.Fatal("native speed cap was not preserved")
	}
	w.ApplyBonusPower(Slow, 3)
	if w.Balls[0].VX != 1 || w.Balls[0].VY != -1 {
		t.Fatal("native minimum speed was not preserved")
	}
}

func TestSuperBallAndGhostBallUseDifferentCollisionRules(t *testing.T) {
	for _, effect := range []EffectKind{SuperBall, GhostBall} {
		w := testWorld()
		startWorld(w)
		w.Balls = []Ball{{ID: 1, X: 128, Y: 25, VY: 20, Radius: NormalBallRadius}}
		w.ApplyBonus(effect)
		w.Tick(Input{})
		if w.Balls[0].VY != 20 {
			t.Fatal("special ball reflected from a normal brick")
		}
		if effect == SuperBall && !w.Bricks[0].Destroyed {
			t.Fatal("super ball did not break the brick")
		}
		if effect == GhostBall && w.Bricks[0].Destroyed {
			t.Fatal("ghost ball should bypass the brick")
		}
	}
}

func TestEnemyShieldUsesCharges(t *testing.T) {
	w := testWorld()
	startWorld(w)
	w.ApplyBonus(Shield)
	w.Enemies = []Enemy{{X: 150, Y: 185, W: 16, H: 23, Kind: 2, Code: 0x9002, HP: 1}}
	w.Tick(Input{})
	if w.Lives != InitialLives || w.ShieldCharges != 0 || w.Active(Shield) {
		t.Fatal("shield did not absorb exactly one lethal contact")
	}
}

func TestOriginalScoreCannotBecomeNegative(t *testing.T) {
	w := testWorld()
	w.Score, w.ScoreMultiplier = 65530, 0
	w.damageBrick(0)
	if w.Score != 4 {
		t.Fatalf("native 16-bit score did not wrap: %d", w.Score)
	}
	w.ScoreMultiplier = 65535
	w.ApplyBonusPower(ScoreBonus, 0)
	if w.ScoreMultiplier != 0 {
		t.Fatal("native multiplier word did not wrap")
	}
}

func TestMouseSensitivityUsesMovementWithoutOscillation(t *testing.T) {
	w := testWorld()
	startWorld(w)
	w.ApplyBonus(MouseSensitivity)
	w.Tick(Input{UseMouse: true, PaddleX: 160, PaddleY: 189})
	w.Tick(Input{UseMouse: true, PaddleX: 165, PaddleY: 189})
	if w.Paddle.X != 175 {
		t.Fatalf("original mouse multiplier not applied: %g", w.Paddle.X)
	}
	w.Tick(Input{UseMouse: true, PaddleX: 165, PaddleY: 189})
	if w.Paddle.X != 175 {
		t.Fatal("stationary pointer caused paddle oscillation")
	}
}

func TestReverseMouseDoesNotTeleportPaddleOnActivation(t *testing.T) {
	w := testWorld()
	startWorld(w)
	w.Tick(Input{UseMouse: true, PaddleX: 180, PaddleY: 189})
	w.ApplyBonus(Reverse)
	w.Tick(Input{UseMouse: true, PaddleX: 180, PaddleY: 189})
	if w.Paddle.X != 180 {
		t.Fatal("reverse bonus moved a stationary paddle")
	}
	w.Tick(Input{UseMouse: true, PaddleX: 185, PaddleY: 189})
	if w.Paddle.X != 175 {
		t.Fatal("reverse bonus did not invert horizontal movement")
	}
}

func TestCannonBypassesLayersAndLaserPreservesThem(t *testing.T) {
	for _, cannon := range []bool{false, true} {
		w := New([]Level{{Bricks: []Brick{{X: 120, Y: 40, W: 16, H: 8, Kind: 0x23, HP: 3, Score: 1, Destructible: true}}}}, 1)
		startWorld(w)
		w.Balls = []Ball{{ID: 1, X: 200, Y: 100, VY: -1, Radius: NormalBallRadius}}
		w.Shots = []Shot{{X: 128, Y: 49, VY: -4, Width: 1, Cannon: cannon}}
		w.Tick(Input{})
		if cannon && !w.Bricks[0].Destroyed {
			t.Fatal("native cannon did not bypass reinforcement")
		}
		if !cannon && (w.Bricks[0].Destroyed || w.Bricks[0].HP != 2 || w.Bricks[0].Kind != 0x13) {
			t.Fatal("native laser did not peel one layer")
		}
	}
}

func TestLaserModeControlsWidthAndSpeed(t *testing.T) {
	w := testWorld()
	startWorld(w)
	w.ApplyBonus(WeaponMode8)
	w.Tick(Input{Fire: true})
	if len(w.Shots) != 1 || w.Shots[0].VY != -8 || w.Shots[0].Width != 13 || w.Shots[0].Cannon {
		t.Fatalf("fast double laser mode changed: %+v", w.Shots)
	}
}

func TestRecoveredCampaignReplayKeepsFinitePhysics(t *testing.T) {
	table, err := os.ReadFile("../../assets/original/level.tab")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile("../../assets/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	levels, err := LoadCampaign(table, metadata)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != OriginalLevelCount {
		t.Fatalf("original campaign lost rounds: %d", len(levels))
	}
	w := New(levels, 2026)
	for tick := 0; tick < 6000; tick++ {
		x := w.Paddle.X
		lowest := -1.0
		for _, ball := range w.Balls {
			if ball.Y > lowest {
				x, lowest = ball.X, ball.Y
			}
		}
		w.Tick(Input{UseMouse: true, PaddleX: x, PaddleY: 189, Launch: true, Fire: true})
		if w.Score < 0 || w.Score > 65535 || w.Lives < 0 {
			t.Fatalf("invalid counters at tick%d", tick)
		}
		for _, ball := range w.Balls {
			for _, value := range []float64{ball.X, ball.Y, ball.VX, ball.VY, ball.Radius} {
				if math.IsNaN(value) || math.IsInf(value, 0) {
					t.Fatalf("nonfinite original-campaign physics at tick%d: %+v", tick, ball)
				}
			}
		}
	}
}

func TestPaddleSizeLimitsAndSpecialShrinkTiles(t *testing.T) {
	w := testWorld()
	w.ApplyBonusPower(Grow, 3)
	for w.Paddle.Slot != w.Paddle.TargetSlot {
		w.updatePaddleSize()
	}
	if w.Paddle.Slot != 21 || w.Paddle.W != 66 {
		t.Fatal("growth did not advance eight native slots")
	}
	w.ApplyBonusPower(Grow, 3)
	for w.Paddle.Slot != w.Paddle.TargetSlot {
		w.updatePaddleSize()
	}
	if w.Paddle.Slot != 24 || w.Paddle.W != 78 {
		t.Fatal("native upper paddle limit changed")
	}
	w.Bricks = append(w.Bricks, Brick{Kind: 0xf7})
	w.damageBrick(len(w.Bricks) - 1)
	if w.Paddle.TargetSlot != 18 {
		t.Fatal("special tile247 did not shrink six slots")
	}
	for w.Paddle.Slot != w.Paddle.TargetSlot {
		w.updatePaddleSize()
	}
	for i := 0; i < 4; i++ {
		w.ApplyBonusPower(Shrink, 3)
		for w.Paddle.Slot != w.Paddle.TargetSlot {
			w.updatePaddleSize()
		}
	}
	if w.Paddle.Slot != 7 || w.Paddle.W != 14 {
		t.Fatal("native lower paddle limit changed")
	}
}

func TestSimultaneousCombatDefeatDoesNotGrantVictory(t *testing.T) {
	w := New(make([]Level, 10), 1)
	if err := w.StartCombat(1); err != nil {
		t.Fatal(err)
	}
	w.Combat.PlayerEnergy, w.Combat.BossEnergy = 64, 8
	w.Combat.PlayerShots = []CombatShot{{X: 244, Y: 82, VX: 3}}
	w.Combat.BossShots = []CombatShot{{X: 35, Y: w.Combat.ShipY + 16, VX: -5}}
	w.Tick(Input{})
	if w.State != GameOver || w.Combat.BossEnergy != 0 {
		t.Fatal("player energy must remain positive to defeat the original boss")
	}
}

func TestNativePaddleMotionChangesVXWithArithmeticShift(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		targetX, incomingVX, wantVX float64
	}{
		{"right", 164, 1, 3}, {"left", 157, 1, -1}, {"clamp", 200, 1, 3}, {"zero allowed", 160, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := testWorld()
			startWorld(w)
			w.Balls = []Ball{{ID: 1, X: tc.targetX, Y: 182, VX: tc.incomingVX, VY: 3, Radius: 3}}
			w.Tick(Input{UseMouse: true, PaddleX: tc.targetX, PaddleY: 189})
			if w.Balls[0].VX != tc.wantVX || w.Balls[0].VY != -3 {
				t.Fatalf("native momentum bounce changed: %+v", w.Balls[0])
			}
		})
	}
}

func TestTeleportPlacesBallOutsideExitWithoutCooldown(t *testing.T) {
	w := New([]Level{{Bricks: []Brick{{X: 32, Y: 40, W: 16, H: 8, Kind: 0xf8}, {X: 240, Y: 100, W: 16, H: 8, Kind: 0xf9}}}}, 1)
	ball := Ball{X: 40, Y: 44, VX: 1, VY: -2, Radius: 2.5}
	w.collidePortals(&ball)
	if ball.X != 258.5 || ball.Y != 94.5 {
		t.Fatalf("native teleport offset changed: %+v", ball)
	}
	w.collidePortals(&ball)
	if ball.X != 258.5 || ball.Y != 94.5 {
		t.Fatal("exit cell immediately teleported the ball back")
	}
}

func TestOriginalWeaponShotLimitsAndCadence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		effect EffectKind
		want   int
	}{
		{"normal cannon", Laser, 1}, {"normal laser", WeaponMode1, 1}, {"enhanced cannon", EnhancedCannon, 5}, {"fast laser", WeaponMode4, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := testWorld()
			startWorld(w)
			w.ApplyBonus(tc.effect)
			for i := 0; i < 10; i++ {
				w.Tick(Input{Fire: true})
			}
			if len(w.Shots) != tc.want {
				t.Fatalf("native held-fire count changed: %d", len(w.Shots))
			}
		})
	}
}

func TestCombatPlayerFiresEachUpdateWithSixteenShotCap(t *testing.T) {
	w := New(make([]Level, 10), 1)
	if err := w.StartCombat(1); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		w.Tick(Input{Fire: true})
	}
	if len(w.Combat.PlayerShots) != 16 {
		t.Fatalf("native combat held-fire cadence changed: %d", len(w.Combat.PlayerShots))
	}
}
