package game

import "fmt"

// CombatShot is a projectile in the original horizontal alien encounter.
type CombatShot struct{ X, Y, VX, VY float64 }

// CombatData retains native ship coordinates and both original energy counters.
type CombatData struct {
	Number                   int
	ShipX, ShipY             float64
	PlayerEnergy, BossEnergy int
	BossFrame                int
	PlayerShots, BossShots   []CombatShot
	Tick                     int
}

// SetCombatBoundary installs the original profile, with one entry per four scanlines.
func (w *World) SetCombatBoundary(profile []int) {
	w.combatBoundary = append([]int(nil), profile...)
}

// StartCombat selects one of the six authentic encounters for review or practice.
func (w *World) StartCombat(number int) error {
	if number < 1 || number > 6 {
		return fmt.Errorf("combat %d is outside the original campaign", number)
	}
	if err := w.StartAt(number*10 - 1); err != nil {
		return err
	}
	w.startCombat()
	return nil
}

func (w *World) startCombat() {
	w.State, w.stateTicks, w.combatShotCount, w.combatAimVariant = Combat, 0, 0, false
	w.Combat = &CombatData{Number: (w.LevelIndex + 1) / 10, ShipX: 16, ShipY: 70, PlayerEnergy: 448, BossEnergy: 896}
	w.Balls, w.Drops, w.Shots, w.Enemies = nil, nil, nil, nil
	w.emit(CombatStarted, 16, 70, w.Combat.Number, NoEffect)
}

func (w *World) updateCombat(input Input) {
	combat := w.Combat
	if combat == nil {
		w.startCombat()
		combat = w.Combat
	}
	combat.Tick++
	if input.UseMouse {
		combat.ShipY = clamp(input.PaddleY, 0, 134)
	} else {
		if input.Up {
			combat.ShipY -= 3
		}
		if input.Down {
			combat.ShipY += 3
		}
		combat.ShipY = clamp(combat.ShipY, 0, 134)
	}
	frames := [...]int{0, 1, 2, 1}
	combat.BossFrame = frames[(combat.Tick/6)%len(frames)]
	if (input.Fire || input.Launch) && len(combat.PlayerShots) < 16 {
		combat.PlayerShots = append(combat.PlayerShots, CombatShot{X: 14, Y: combat.ShipY + 12, VX: 3})
		w.emit(ShotFired, 14, combat.ShipY+12, 0, NoEffect)
	}
	if (combat.Tick/6)%4 >= 2 && combat.Tick%4 == 0 {
		w.combatShotCount++
		if w.combatShotCount%3 == 0 {
			w.combatAimVariant = !w.combatAimVariant
		}
		velocityY := float64(int(combat.ShipY+16-82)/23) - 1
		if w.combatAimVariant {
			velocityY += 2
		}
		combat.BossShots = append(combat.BossShots, CombatShot{X: 245, Y: 82, VX: -5, VY: velocityY})
	}
	playerShots := combat.PlayerShots[:0]
	for _, shot := range combat.PlayerShots {
		shot.X += shot.VX
		shot.Y += shot.VY
		boundary := 245
		if y := int(shot.Y) / 4; y >= 0 && y < len(w.combatBoundary) {
			boundary = w.combatBoundary[y]
		}
		if shot.X >= float64(boundary) {
			if shot.Y >= 68 && shot.Y <= 91 {
				damage := 8
				if combat.Number == 6 {
					damage = 4
				}
				combat.BossEnergy = max(0, combat.BossEnergy-damage)
				w.emit(CombatHit, shot.X, shot.Y, damage, NoEffect)
			}
			continue
		}
		if shot.X < Width {
			playerShots = append(playerShots, shot)
		}
	}
	combat.PlayerShots = playerShots
	bossShots := combat.BossShots[:0]
	for _, shot := range combat.BossShots {
		shot.X += shot.VX
		shot.Y += shot.VY
		if shot.X >= 4 && shot.X <= 32 && shot.Y >= combat.ShipY && shot.Y <= combat.ShipY+32 {
			combat.PlayerEnergy = max(0, combat.PlayerEnergy-64)
			w.emit(CombatHit, shot.X, shot.Y, -64, NoEffect)
			continue
		}
		if shot.X >= 0 && shot.Y >= 0 && shot.Y < 167 {
			bossShots = append(bossShots, shot)
		}
	}
	combat.BossShots = bossShots
	if combat.PlayerEnergy <= 0 {
		// The original alien encounter ends the run even if brick-round reserves remain.
		w.State = GameOver
		w.emit(LifeLost, combat.ShipX, combat.ShipY, 0, NoEffect)
		return
	}
	if combat.BossEnergy <= 0 {
		w.emit(CombatCompleted, 245, 82, combat.Number, NoEffect)
		if w.LevelIndex+1 >= len(w.Levels) {
			w.State = Won
		} else {
			w.loadLevel(w.LevelIndex + 1)
		}
	}
}
