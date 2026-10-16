package game

// EnableUnlimitedReserves enables the original optional, process-persistent
// reserve cheat. The presentation must authorize this API through manual input;
// expert validation and recorded gameplay never call cheat methods.
func (w *World) EnableUnlimitedReserves() { w.UnlimitedReserves = true }

// CheatSkipRound follows the normal round transition without awarding points.
// Every tenth round still enters its authentic alien encounter.
func (w *World) CheatSkipRound() {
	switch w.State {
	case Ready, Playing, LevelClear, Dying:
	default:
		return
	}
	if len(w.Levels) == 0 {
		return
	}
	w.DeathSlot, w.DeathTicks = 0, 0
	w.advanceRound()
}

// CheatFinalCombat enters the original sixth encounter with existing reserves
// and score. It does not weaken the boss or protect the player's combat energy.
func (w *World) CheatFinalCombat() {
	if len(w.Levels) < OriginalLevelCount {
		return
	}
	w.LevelIndex = OriginalLevelCount - 1
	w.DeathSlot, w.DeathTicks = 0, 0
	w.startCombat()
}

// CheatWeakenBoss applies the original optional F8 energy adjustment. Actual
// player shots are still required to defeat the encounter.
func (w *World) CheatWeakenBoss() {
	if w.State == Combat && w.Combat != nil {
		w.Combat.BossEnergy = 8
	}
}

func (w *World) advanceRound() {
	if (w.LevelIndex+1)%10 == 0 {
		w.startCombat()
		return
	}
	if w.LevelIndex+1 >= len(w.Levels) {
		w.State = Won
		return
	}
	w.loadLevel(w.LevelIndex + 1)
}
