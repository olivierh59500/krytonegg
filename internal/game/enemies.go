package game

import "math"

var enemyDimensions = [...]struct{ W, H float64 }{{13, 13}, {12, 25}, {16, 23}, {15, 15}, {19, 22}, {9, 18}}

func (w *World) resetEnemyTiming() {
	w.enemyCooldown, w.enemyDoorPhase, w.enemyDoorTicks = 1, 0, 0
	w.enemyAnimationFrame = 0
}

// EnemyTurnCounter returns the native motion clock without exposing mutable state.
func (w *World) EnemyTurnCounter() int { return w.enemyTurnCounter }

// AdvanceEnemyKinematics predicts one creature using the shared native counter.
// Returned values are copies; neither the supplied creature nor a World changes.
func AdvanceEnemyKinematics(enemy Enemy, turnCounter int) (Enemy, int) {
	turnCounter--
	if turnCounter <= 0 && !enemy.Destroyed {
		turnCounter = 169
		enemy.VY = -enemy.VY
	}
	enemy.Age++
	enemy.X += enemy.VX
	enemy.Y += enemy.VY
	if enemy.X < 20 {
		enemy.X, enemy.VX = 20, -enemy.VX
	}
	if enemy.X > 295-enemy.W {
		enemy.X, enemy.VX = 295-enemy.W, -enemy.VX
	}
	if enemy.Y < 28 && enemy.VY < 0 {
		enemy.Y, enemy.VY = 28, -enemy.VY
	}
	if enemy.Y > 194-enemy.H {
		enemy.Y, enemy.VY = 194-enemy.H, -enemy.VY
	}
	return enemy, turnCounter
}

func (w *World) updateEnemyDoor() {
	if len(w.Levels[w.LevelIndex].EnemyChoices) == 0 {
		return
	}
	if w.enemyDoorPhase == 0 {
		if len(w.Enemies) >= 8 {
			return
		}
		w.enemyCooldown++
		if w.enemyCooldown < 76 {
			return
		}
		w.enemyCooldown, w.enemyDoorPhase, w.enemyDoorTicks = 0, 1, 0
	}
	if w.enemyDoorPhase == 1 {
		w.DoorFrame = min(10, w.enemyDoorTicks/2)
		w.enemyDoorTicks++
		if w.enemyDoorTicks == 22 {
			w.enemyDoorPhase, w.enemyDoorTicks = 2, 0
		}
		return
	}
	if w.enemyDoorTicks == 0 && len(w.Enemies) < 8 {
		choices := w.Levels[w.LevelIndex].EnemyChoices
		left := int(w.Paddle.X - w.Paddle.W/2)
		code := choices[(left&3)%len(choices)]
		kind := int(code & 0xff)
		if kind >= 0 && kind < len(enemyDimensions) {
			size := enemyDimensions[kind]
			vx := 1.0
			if left&1 != 0 {
				vx = -1
			}
			w.Enemies = append(w.Enemies, Enemy{X: 146, Y: 14, VX: vx, VY: 1, W: size.W, H: size.H, Kind: kind, HP: 1, Code: code, AnimationDirection: 1})
			w.emit(EnemySpawned, 146, 14, kind, NoEffect)
		}
	}
	w.DoorFrame = max(0, 10-w.enemyDoorTicks/2)
	w.enemyDoorTicks++
	if w.enemyDoorTicks >= 22 {
		w.enemyDoorPhase, w.enemyDoorTicks, w.enemyCooldown, w.DoorFrame = 0, 0, 1, 0
	}
}

func (w *World) updateEnemies() {
	w.updateEnemyDoor()
	if len(w.Enemies) == 0 {
		return
	}
	w.enemyAnimationCounter--
	if w.enemyAnimationCounter < 0 {
		w.enemyAnimationCounter = 8
	}
	animate := w.enemyAnimationCounter == 0
	live := w.Enemies[:0]
	for _, source := range w.Enemies {
		enemy := source
		if enemy.Destroyed && enemy.DeathTicks <= 0 {
			continue
		}
		enemy, w.enemyTurnCounter = AdvanceEnemyKinematics(enemy, w.enemyTurnCounter)
		if enemy.Destroyed {
			enemy.DeathTicks--
			elapsed := 56 - enemy.DeathTicks
			if elapsed <= 28 {
				enemy.DeathFrame = 20 - elapsed/2
			} else {
				enemy.DeathFrame = 6 - (elapsed-28)/4
			}
			if enemy.DeathTicks > 0 {
				live = append(live, enemy)
			}
			continue
		}
		if animate {
			advanceEnemyAnimation(&enemy)
		}
		if w.State != Dying && rectanglesOverlap(enemy.X, enemy.Y, enemy.W, enemy.H, w.Paddle.X-w.Paddle.W/2, w.Paddle.Y-w.Paddle.H/2, w.Paddle.W, w.Paddle.H) {
			w.beginEnemyDeath(&enemy)
			if enemy.Code&0x1000 != 0 {
				if w.ShieldCharges > 0 {
					w.ShieldCharges--
					if w.ShieldCharges == 0 {
						w.removeEffect(Shield)
					}
				} else {
					w.Balls = nil
				}
			}
		}
		live = append(live, enemy)
	}
	w.Enemies = live
}

func advanceEnemyAnimation(enemy *Enemy) {
	count := 3
	if enemy.Kind == 0 {
		count = 2
	}
	if enemy.AnimationDirection == 0 {
		enemy.AnimationDirection = 1
	}
	enemy.Frame = max(0, min(count-1, enemy.AnimationStep))
	if enemy.Code&0x8000 != 0 {
		if enemy.AnimationStep >= count-1 {
			enemy.AnimationDirection = -1
		}
		if enemy.AnimationStep <= 0 {
			enemy.AnimationDirection = 1
		}
		enemy.AnimationStep += enemy.AnimationDirection
	} else {
		enemy.AnimationStep = (enemy.AnimationStep + 1) % count
	}
	enemy.DrawOffsetX, enemy.DrawOffsetY = 0, 0
	switch enemy.Kind {
	case 0:
		enemy.DrawOffsetX, enemy.DrawOffsetY = -1, -2
	case 1:
		if enemy.Frame == 0 {
			enemy.DrawOffsetY = -9
		} else {
			enemy.DrawOffsetX, enemy.DrawOffsetY = -5, -1
		}
	case 2:
		enemy.DrawOffsetX = 2
	case 3:
		if enemy.Frame != 0 {
			enemy.DrawOffsetX, enemy.DrawOffsetY = -1, -1
		}
	case 5:
		if enemy.Frame == 0 {
			enemy.DrawOffsetY = 1
		} else {
			enemy.DrawOffsetX = -1
		}
	}
}

func (w *World) beginEnemyDeath(enemy *Enemy) {
	if enemy.Destroyed {
		return
	}
	enemy.Destroyed, enemy.DeathFrame, enemy.DeathTicks = true, 20, 56
	w.soundEvent(EnemyHit, enemy.X+enemy.W/2, enemy.Y+enemy.H/2, enemy.Kind, NoEffect, "enemy-hit", 0x28a)
}

func rectanglesOverlap(ax, ay, aw, ah, bx, by, bw, bh float64) bool {
	return ax < bx+bw && ax+aw > bx && ay < by+bh && ay+ah > by
}

func (w *World) collideEnemies(ball *Ball) {
	if w.Active(GhostBall) {
		return
	}
	for i := range w.Enemies {
		enemy := &w.Enemies[i]
		if enemy.Destroyed {
			continue
		}
		_, _, _, collision := circleRect(ball.X, ball.Y, ball.Radius, enemy.X, enemy.Y, enemy.W, enemy.H)
		if !collision {
			continue
		}
		w.beginEnemyDeath(enemy)
		if !w.Active(SuperBall) && !ball.Attached {
			ball.VY = -ball.VY
			ball.VX = math.Copysign(float64((w.FrameCounter&3)|1), ball.VX)
		}
	}
}
