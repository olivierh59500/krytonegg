package game

import "testing"

func TestOriginalCheatsDefaultOffAndExpertInputCannotEnableThem(t *testing.T) {
	w := testWorld()
	if w.UnlimitedReserves {
		t.Fatal("original optional cheats were enabled by default")
	}
	startWorld(w)
	w.Balls = nil
	w.Tick(Input{})
	for tick := 0; tick < 14; tick++ {
		w.Tick(Input{})
	}
	if w.Lives != InitialLives-1 {
		t.Fatal("normal input unexpectedly bypassed reserve deduction")
	}
}

func TestOriginalUnlimitedReservesPersistButDoNotProtectCombat(t *testing.T) {
	w := testWorld()
	startWorld(w)
	w.EnableUnlimitedReserves()
	w.Balls = nil
	w.Tick(Input{})
	for tick := 0; tick < 14; tick++ {
		w.Tick(Input{})
	}
	if w.State != Ready || w.Lives != InitialLives {
		t.Fatal("optional unlimited reserves did not survive normal ship death")
	}
	w.Restart()
	if !w.UnlimitedReserves {
		t.Fatal("native persistent reserve cheat reset across games")
	}
	c := New(make([]Level, OriginalLevelCount), 1)
	c.PreserveRuntimeFrom(w)
	if !c.UnlimitedReserves {
		t.Fatal("app world factory dropped the native reserve cheat")
	}
	c.CheatFinalCombat()
	c.Combat.PlayerEnergy = 64
	c.Combat.BossShots = []CombatShot{{X: 35, Y: c.Combat.ShipY + 16, VX: -5}}
	c.Tick(Input{})
	if c.State != GameOver || c.Combat.PlayerEnergy != 0 {
		t.Fatal("reserve cheat incorrectly protected combat energy")
	}
}

func TestOriginalOptionalSkipStillEntersEveryTenthCombat(t *testing.T) {
	w := New(make([]Level, OriginalLevelCount), 1)
	if err := w.StartAt(8); err != nil {
		t.Fatal(err)
	}
	w.Score, w.Lives = 1234, 7
	w.CheatSkipRound()
	if w.LevelIndex != 9 || w.State != Ready || w.Score != 1234 || w.Lives != 7 {
		t.Fatal("optional skip reset or awarded score/reserves")
	}
	w.CheatSkipRound()
	if w.State != Combat || w.Combat.Number != 1 {
		t.Fatal("optional skip bypassed the authentic tenth-round boss")
	}
	w.CheatFinalCombat()
	if w.LevelIndex != 59 || w.Combat.Number != 6 || w.Score != 1234 || w.Lives != 7 {
		t.Fatal("optional final-combat jump changed normal counters")
	}
	w.CheatWeakenBoss()
	if w.Combat.BossEnergy != 8 || w.Combat.PlayerEnergy != 448 || w.State != Combat {
		t.Fatal("optional boss adjustment granted automatic victory or player protection")
	}
}
