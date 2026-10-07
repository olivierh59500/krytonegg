package game

import (
	"math"
	"testing"
)

func TestRecoveredAdditionalBallStartsFreshAtPaddle(t *testing.T) {
	w := testWorld()
	startWorld(w)
	w.Balls[0] = Ball{ID: 1, X: 60, Y: 80, VX: -3, VY: 4, Radius: 8, Attached: true}
	w.ApplyBonus(Multiball)
	if len(w.Balls) != 2 {
		t.Fatalf("additional ball count %d", len(w.Balls))
	}
	b := w.Balls[1]
	if b.ID == 1 || b.Attached || b.Radius != 2.5 || b.VX != 1 || b.VY != -2 {
		t.Fatalf("native additional ball changed: %+v", b)
	}
	if b.X-b.Radius != w.Paddle.X-w.Paddle.W/2+15 || b.Y-b.Radius != w.Paddle.Y-w.Paddle.H/2-4 {
		t.Fatalf("native additional-ball origin changed: %+v", b)
	}
	for len(w.Balls) < 8 {
		w.ApplyBonus(Multiball)
	}
	w.ApplyBonus(Multiball)
	if len(w.Balls) != 8 {
		t.Fatal("additional-ball capacity changed")
	}
}

func TestRecoveredEnemyCollisionKeepsIntegralVelocity(t *testing.T) {
	for _, tick := range []uint64{0, 1, 2, 3, 4, 5} {
		for _, vx := range []float64{-2, 0, 2} {
			w := testWorld()
			w.FrameCounter = tick
			w.Enemies = []Enemy{{X: 100, Y: 80, W: 16, H: 23, VX: 1, VY: 1}}
			ball := Ball{X: 108, Y: 85, VX: vx, VY: 2, Radius: 2.5}
			w.collideEnemies(&ball)
			want := math.Copysign(float64((tick&3)|1), vx)
			if ball.VX != want || ball.VY != -2 || !w.Enemies[0].Destroyed || w.Enemies[0].DeathTicks != 56 {
				t.Fatalf("native enemy collision tick %d: %+v", tick, ball)
			}
		}
	}
	for _, attached := range []bool{false, true} {
		w := testWorld()
		w.Enemies = []Enemy{{X: 100, Y: 80, W: 16, H: 23}}
		ball := Ball{X: 108, Y: 85, VX: -2, VY: 2, Radius: 2.5, Attached: attached}
		if !attached {
			w.ApplyBonus(SuperBall)
		}
		w.collideEnemies(&ball)
		if ball.VX != -2 || ball.VY != 2 || !w.Enemies[0].Destroyed {
			t.Fatal("super or attached ball received native enemy rebound")
		}
	}
}

func TestRecoveredGlobalEnemyTurnIncludesDeathSlots(t *testing.T) {
	w := testWorld()
	w.enemyTurnCounter = 2
	w.Enemies = []Enemy{{X: 70, Y: 80, VX: 1, VY: 1, W: 16, H: 23, Destroyed: true, DeathTicks: 56}, {X: 120, Y: 80, VX: 1, VY: 1, W: 16, H: 23}}
	w.updateEnemies()
	if w.enemyTurnCounter != 169 || w.Enemies[1].VY != -1 || w.Enemies[1].VX != 1 {
		t.Fatalf("global native turn changed: %+v", w.Enemies)
	}
}

func TestRecoveredEnemyDeathRetainsCapacityFor56Updates(t *testing.T) {
	w := testWorld()
	w.Enemies = []Enemy{{X: 100, Y: 70, VX: 1, VY: 1, W: 16, H: 23}}
	w.beginEnemyDeath(&w.Enemies[0])
	for tick := 1; tick <= 55; tick++ {
		w.updateEnemies()
		if len(w.Enemies) != 1 || w.Enemies[0].DeathTicks != 56-tick {
			t.Fatalf("native enemy removed early at %d", tick)
		}
	}
	w.updateEnemies()
	if len(w.Enemies) != 0 {
		t.Fatal("native enemy death did not finish at 56 updates")
	}
}

func TestRecoveredEnemyAnimationUsesSharedNineUpdateClock(t *testing.T) {
	w := testWorld()
	w.Enemies = []Enemy{{X: 70, Y: 70, W: 12, H: 25, Kind: 1, Code: 0x8001, AnimationDirection: 1}}
	w.updateEnemies()
	if w.Enemies[0].Frame != 0 || w.Enemies[0].DrawOffsetY != -9 || w.Enemies[0].Y != 70 {
		t.Fatal("draw offsets changed physical enemy coordinates")
	}
	for tick := 0; tick < 8; tick++ {
		w.updateEnemies()
		if w.Enemies[0].Frame != 0 {
			t.Fatal("enemy animation advanced before nine updates")
		}
	}
	w.updateEnemies()
	if w.Enemies[0].Frame != 1 || w.Enemies[0].DrawOffsetX != -5 {
		t.Fatal("global nine-update animation did not advance")
	}
	for _, want := range []int{2, 1, 0} {
		for tick := 0; tick < 9; tick++ {
			w.updateEnemies()
		}
		if w.Enemies[0].Frame != want {
			t.Fatal("encoded enemy animation did not ping-pong")
		}
	}
}

func TestRecoveredDoorWaitAndSpawnParity(t *testing.T) {
	w := testWorld()
	w.Levels[0].EnemyChoices = []uint16{0, 0, 0, 0}
	w.Paddle.X = 161
	for tick := 0; tick < 96; tick++ {
		w.updateEnemyDoor()
		if len(w.Enemies) != 0 {
			t.Fatalf("enemy spawned before native waiting/opening sequence: %d", tick)
		}
	}
	w.updateEnemyDoor()
	if len(w.Enemies) != 1 || w.Enemies[0].X != 146 || w.Enemies[0].Y != 14 || w.Enemies[0].VX != -1 {
		t.Fatalf("native spawning or paddle parity changed: %+v", w.Enemies)
	}
	for len(w.Enemies) < 8 {
		w.Enemies = append(w.Enemies, Enemy{Destroyed: true, DeathTicks: 56})
	}
	for w.enemyDoorPhase != 0 {
		w.updateEnemyDoor()
	}
	clock := w.enemyCooldown
	for tick := 0; tick < 100; tick++ {
		w.updateEnemyDoor()
	}
	if w.enemyCooldown != clock {
		t.Fatal("full enemy capacity advanced native waiting counter")
	}
}

func TestRecoveredSuperBallRemovesPermanentTilesAndAllLayers(t *testing.T) {
	w := testWorld()
	w.Bricks = []Brick{{X: 80, Y: 48, W: 16, H: 8, Kind: 0xf4, HP: -1}, {X: 112, Y: 48, W: 16, H: 8, Kind: 0x23, HP: 3, Destructible: true}}
	w.damageSuperBrick(0)
	w.damageSuperBrick(1)
	if !w.Bricks[0].Destroyed || !w.Bricks[1].Destroyed || w.Bricks[1].HP != 0 {
		t.Fatal("super ball did not remove complete native tile")
	}
}

func TestRecoveredCombatMouthWaitAndClosingFireFlag(t *testing.T) {
	w := New(make([]Level, 10), 1)
	if err := w.StartCombat(1); err != nil {
		t.Fatal(err)
	}
	if w.Combat.ShipY != 75 {
		t.Fatal("native combat starting ship height changed")
	}
	for tick := 0; tick < 223; tick++ {
		w.updateCombatMouth()
		if w.Combat.BossFrame != 0 {
			t.Fatal("mouth opened before HP-derived threshold")
		}
	}
	for tick := 0; tick < 6; tick++ {
		w.updateCombatMouth()
	}
	if w.Combat.BossFrame != 1 {
		t.Fatal("mouth did not begin its six-update opening step")
	}
	for tick := 0; tick < 6; tick++ {
		w.updateCombatMouth()
	}
	if w.Combat.BossFrame != 2 || !w.Combat.MouthOpen {
		t.Fatal("mouth did not activate native fire flag")
	}
	for tick := 0; tick < 6; tick++ {
		w.updateCombatMouth()
	}
	if w.Combat.BossFrame != 1 || !w.Combat.MouthOpen {
		t.Fatal("closing frame cleared native fire flag early")
	}
	for tick := 0; tick < 6; tick++ {
		w.updateCombatMouth()
	}
	if w.Combat.BossFrame != 0 || w.Combat.MouthOpen {
		t.Fatal("closed mouth retained native fire flag")
	}
}

func TestRecoveredBossShotAimAndHalfRateVerticalMovement(t *testing.T) {
	w := New(make([]Level, 10), 1)
	if err := w.StartCombat(1); err != nil {
		t.Fatal(err)
	}
	w.Combat.BossFrame, w.Combat.MouthOpen, w.Combat.mouthActive, w.Combat.mouthTimer = 2, true, true, -100
	for index, want := range []float64{0, 1, 0, -1} {
		for tick := 0; tick < 4; tick++ {
			w.Tick(Input{})
		}
		if got := w.Combat.BossShots[index].VY; got != want {
			t.Fatalf("native boss aim index %d = %g, want %g", index, got, want)
		}
	}
	w.Combat.BossShots = []CombatShot{{X: 100, Y: 50, VX: -5, VY: 2}}
	w.Combat.MouthOpen = false
	w.Tick(Input{})
	if w.Combat.BossShots[0].Y != 50 {
		t.Fatal("native boss Y moved on odd update")
	}
	w.Tick(Input{})
	if w.Combat.BossShots[0].Y != 52 || w.Combat.BossShots[0].X != 90 {
		t.Fatal("native boss alternate Y / continuous X movement changed")
	}
}

func TestRecoveredScoreThresholdAwardsOneReservePerScoringEvent(t *testing.T) {
	w := testWorld()
	w.Score, w.ScoreMultiplier = 2040, 1
	w.damageBrick(0)
	if w.Score != 2060 || w.Lives != 6 || w.scoreLifeThreshold != 1 {
		t.Fatal("native 2048 threshold did not award a reserve")
	}
	w.Bricks[0].Destroyed, w.Bricks[0].HP = false, 1
	w.ScoreMultiplier = 10
	w.damageBrick(0)
	if w.Lives != 7 || w.scoreLifeThreshold != 6 {
		t.Fatal("native threshold jump must award one reserve, not one per crossed interval")
	}
	w.Restart()
	if w.scoreLifeThreshold != 6 {
		t.Fatal("native reserve threshold unexpectedly reset across games")
	}
}

func TestRecoveredMultiplierResetsOnRoundAndDelayedRespawn(t *testing.T) {
	w := testWorld()
	w.ApplyBonusPower(ScoreBonus, 3)
	w.loadLevel(0)
	if w.ScoreMultiplier != 0 {
		t.Fatal("native score multiplier survived a new round")
	}
	startWorld(w)
	w.ApplyBonusPower(ScoreBonus, 3)
	w.Balls = nil
	w.Tick(Input{})
	if w.State != Dying || w.DeathTicks != 14 || w.Lives != 5 {
		t.Fatal("native default ship death did not preserve reserves during animation")
	}
	for tick := 0; tick < 13; tick++ {
		w.Tick(Input{})
		if w.State != Dying || w.Lives != 5 {
			t.Fatal("native reserve deducted before ship death completed")
		}
	}
	w.Tick(Input{})
	if w.State != Ready || w.Lives != 4 || w.ScoreMultiplier != 0 {
		t.Fatal("native delayed respawn did not reset multiplier")
	}
}

func TestRecoveredReadyShipUpdatesNativeEnemyAndAttachedContacts(t *testing.T) {
	w := testWorld()
	w.State = Ready
	w.Enemies = []Enemy{{X: w.Balls[0].X - 3, Y: w.Balls[0].Y - 5, W: 13, H: 13, VX: 1, VY: 1}}
	w.Tick(Input{})
	if w.State != Ready || !w.Balls[0].Attached || !w.Enemies[0].Destroyed || w.Enemies[0].Age != 1 {
		t.Fatal("attached native ready ship did not update or destroy contacting enemy")
	}
}

func TestRecoveredEnemyClocksContinueAcrossRoundChanges(t *testing.T) {
	w := testWorld()
	w.enemyTurnCounter, w.enemyAnimationCounter = 40, 6
	w.loadLevel(0)
	if w.enemyTurnCounter != 40 || w.enemyAnimationCounter != 6 || w.enemyCooldown != 1 {
		t.Fatal("global native enemy clocks reset on a new round")
	}
}

func TestRecoveredCannonPiercesSteelAndCompleteLayers(t *testing.T) {
	w := New([]Level{{Bricks: []Brick{{X: 112, Y: 80, W: 16, H: 8, Kind: 0x66, HP: -1}, {X: 112, Y: 64, W: 16, H: 8, Kind: 0x23, HP: 3, Destructible: true}, {X: 200, Y: 40, W: 16, H: 8, Kind: 1, HP: 1, Destructible: true}}}}, 1)
	w.Shots = []Shot{{X: 120, Y: 92, VY: -4, Width: 13, Cannon: true}}
	for tick := 0; tick < 8; tick++ {
		w.updateShots()
	}
	if !w.Bricks[0].Destroyed || !w.Bricks[1].Destroyed || len(w.Shots) != 1 {
		t.Fatal("native cannon failed to pierce steel and complete layers")
	}
}

func TestRecoveredRandomBonusUsesPointerAndNativeFrame(t *testing.T) {
	w := testWorld()
	w.Paddle.X = 30
	w.Paddle.W = 36
	w.FrameCounter = 0
	// Paddle left 12 selects the explicitly substituted extra-ball case.
	w.ApplyBonus(RandomBonus)
	if len(w.Balls) != 2 || w.Balls[1].Attached {
		t.Fatal("native random bonus did not substitute extra ball for selector 12")
	}
}

func TestProcessWideNativeCountersCanSurviveAnAppWorldFactory(t *testing.T) {
	a, b := testWorld(), testWorld()
	a.enemyTurnCounter, a.enemyAnimationCounter, a.scoreLifeThreshold = 47, 3, 8
	b.PreserveRuntimeFrom(a)
	if b.enemyTurnCounter != 47 || b.enemyAnimationCounter != 3 || b.scoreLifeThreshold != 8 {
		t.Fatal("app world replacement dropped process-wide native counters")
	}
	b.Restart()
	if b.enemyTurnCounter != 47 || b.enemyAnimationCounter != 3 || b.scoreLifeThreshold != 8 {
		t.Fatal("new game reset process-wide native counters")
	}
}

func TestRecoveredGhostBallBypassesPortalsButSuperBallTeleports(t *testing.T) {
	for _, effect := range []EffectKind{GhostBall, SuperBall} {
		w := testWorld()
		w.Portals = []Portal{{X: 40, Y: 44, Kind: 0xf8}, {X: 248, Y: 104, Kind: 0xf9}}
		w.ApplyBonus(effect)
		ball := Ball{X: 40, Y: 44, VX: 1, VY: -2, Radius: 2.5}
		w.collidePortals(&ball)
		if effect == GhostBall && (ball.X != 40 || ball.Y != 44) {
			t.Fatal("native ghost ball entered the skipped portal collision path")
		}
		if effect == SuperBall && (ball.X == 40 || ball.Y == 44) {
			t.Fatal("native super ball did not enter the portal collision path")
		}
	}
}
