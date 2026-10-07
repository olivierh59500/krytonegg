package game

import "fmt"

// CombatShot is a projectile in the original horizontal alien encounter.
type CombatShot struct{ X, Y, VX, VY float64 }

// CombatData retains native ship coordinates and both original energy counters.
type CombatData struct {
	Number                                  int
	ShipX, ShipY                            float64
	PlayerEnergy, BossEnergy                int
	BossFrame                               int
	MouthOpen                               bool
	PlayerShots, BossShots                  []CombatShot
	Tick                                    int
	closedTicks, mouthTimer, mouthDirection int
	mouthActive                             bool
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
	w.FrameCounter = 0
	w.State, w.stateTicks, w.combatShotCount, w.combatAimVariant = Combat, 0, 0, false
	w.Combat = &CombatData{Number: (w.LevelIndex + 1) / 10, ShipX: 16, ShipY: 75, PlayerEnergy: 448, BossEnergy: 896, mouthDirection: 1}
	w.Balls, w.Drops, w.Shots, w.Enemies = nil, nil, nil, nil
	w.soundEvent(CombatStarted, 16, 75, w.Combat.Number, NoEffect, "combat-end", 0x316)
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
	w.updateCombatMouth()
	if (input.Fire || input.Launch) && len(combat.PlayerShots) < 16 {
		combat.PlayerShots = append(combat.PlayerShots, CombatShot{X: 14, Y: combat.ShipY + 12, VX: 3})
		w.soundEvent(ShotFired, 14, combat.ShipY+12, 0, NoEffect, "combat-fire", 0x244)
	}
	if combat.MouthOpen && combat.Tick%4 == 0 {
		w.combatShotCount++
		if w.combatShotCount%2 == 0 {
			w.combatAimVariant = !w.combatAimVariant
		}
		velocityY := float64(int(combat.ShipY+16-82) / 23)
		if w.combatShotCount%2 == 0 {
			if w.combatAimVariant {
				velocityY++
			} else {
				velocityY--
			}
		}
		combat.BossShots = append(combat.BossShots, CombatShot{X: 245, Y: 82, VX: -5, VY: velocityY})
		w.soundEvent(ShotFired, 245, 82, 1, NoEffect, "enemy-fire", 0x31b)
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
				w.soundEvent(CombatHit, shot.X, shot.Y, damage, NoEffect, "enemy-hit", 0x280)
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
		if combat.Tick%2 == 0 {
			shot.Y += shot.VY
		}
		if shot.X > 4 && shot.X <= 32 && shot.Y+12 >= combat.ShipY && shot.Y <= combat.ShipY+32 {
			combat.PlayerEnergy = max(0, combat.PlayerEnergy-64)
			w.soundEvent(CombatHit, shot.X, shot.Y, -64, NoEffect, "enemy-hit", 0x280)
			continue
		}
		if shot.X > 4 && shot.Y >= 0 && shot.Y <= 157 {
			bossShots = append(bossShots, shot)
		}
	}
	combat.BossShots = bossShots
	if combat.PlayerEnergy <= 0 {
		// The original alien encounter ends the run even if brick-round reserves remain.
		w.State = GameOver
		w.soundEvent(LifeLost, combat.ShipX, combat.ShipY, 0, NoEffect, "game-over", 0x2cb)
		return
	}
	if combat.BossEnergy <= 0 {
		w.soundEvent(CombatCompleted, 245, 82, combat.Number, NoEffect, "game-over", 0x2cb)
		if w.LevelIndex+1 >= len(w.Levels) {
			w.State = Won
		} else {
			w.loadLevel(w.LevelIndex + 1)
		}
	}
}

func (w *World) updateCombatMouth() {
	c := w.Combat
	if !c.mouthActive {
		c.closedTicks++
		if c.closedTicks < max(13, c.BossEnergy>>2) {
			return
		}
		c.closedTicks = 0
		// The source updates the newly active phase on this same PAL update.
		c.mouthDirection = 1
		c.mouthActive = true
	}
	c.mouthTimer++
	if c.mouthTimer <= 5 {
		return
	}
	c.mouthTimer = 0
	c.BossFrame += c.mouthDirection
	if c.BossFrame == 0 {
		c.MouthOpen = false
		c.mouthActive = false
		c.mouthDirection = 1
		return
	}
	if c.BossFrame == 2 {
		c.MouthOpen = true
		d := -((57 - (c.BossEnergy >> 4)) >> 1)
		d += d >> 1
		if c.Number != 6 {
			d >>= 2
		}
		c.mouthTimer = d
		c.mouthDirection = -1
	}
}
