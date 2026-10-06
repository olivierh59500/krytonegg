package game

import "math"

var enemyDimensions = [...]struct{ W, H float64 }{{13, 13}, {12, 25}, {16, 23}, {15, 15}, {19, 22}, {9, 18}}

func (w *World) updateEnemies() {
	choices := w.Levels[w.LevelIndex].EnemyChoices
	if len(choices) > 0 {
		w.enemyCooldown--
		w.DoorFrame = 0
		if w.enemyCooldown > 0 && w.enemyCooldown < 11 {
			w.DoorFrame = 11 - w.enemyCooldown
		}
		if w.enemyCooldown <= 0 {
			w.enemyCooldown = 87
			if len(w.Enemies) < 8 {
				column := int(w.Paddle.X-w.Paddle.W/2) % 4
				if column < 0 {
					column += 4
				}
				code := choices[column%len(choices)]
				kind := int(code & 0xff)
				if kind >= 0 && kind < len(enemyDimensions) {
					size := enemyDimensions[kind]
					w.Enemies = append(w.Enemies, Enemy{X: 146, Y: 14, VX: 1, VY: 1, W: size.W, H: size.H, Kind: kind, HP: 1, Code: code})
					w.emit(EnemySpawned, 146, 14, kind, NoEffect)
				}
			}
		}
	}
	live := w.Enemies[:0]
	for _, enemy := range w.Enemies {
		if enemy.Destroyed {
			continue
		}
		enemy.Age++
		enemy.Frame = (enemy.Age / 6) % 3
		if enemy.Kind == 0 {
			enemy.Frame %= 2
		}
		if enemy.Code&0x8000 != 0 {
			if enemy.Kind == 0 {
				enemy.Frame = 1 - enemy.Frame
			} else {
				enemy.Frame = 2 - enemy.Frame
			}
		}
		if enemy.Age%169 == 0 {
			if (enemy.Age/169)%2 == 0 {
				enemy.VX = -enemy.VX
			} else {
				enemy.VY = -enemy.VY
			}
		}
		enemy.X += enemy.VX
		enemy.Y += enemy.VY
		if enemy.X < 20 {
			enemy.X, enemy.VX = 20, math.Abs(enemy.VX)
		}
		if enemy.X > 295-enemy.W {
			enemy.X, enemy.VX = 295-enemy.W, -math.Abs(enemy.VX)
		}
		if enemy.Y < 28 && enemy.VY < 0 {
			enemy.Y, enemy.VY = 28, math.Abs(enemy.VY)
		}
		if enemy.Y > 194-enemy.H {
			enemy.Y, enemy.VY = 194-enemy.H, -math.Abs(enemy.VY)
		}
		if rectanglesOverlap(enemy.X, enemy.Y, enemy.W, enemy.H, w.Paddle.X-w.Paddle.W/2, w.Paddle.Y-w.Paddle.H/2, w.Paddle.W, w.Paddle.H) {
			w.emit(EnemyHit, enemy.X, enemy.Y, enemy.Kind, NoEffect)
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
			continue
		}
		live = append(live, enemy)
	}
	w.Enemies = live
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
		nx, ny, depth, collision := circleRect(ball.X, ball.Y, ball.Radius, enemy.X, enemy.Y, enemy.W, enemy.H)
		if !collision {
			continue
		}
		ball.X += nx * (depth + 0.001)
		ball.Y += ny * (depth + 0.001)
		approach := ball.VX*nx + ball.VY*ny
		if approach < 0 {
			ball.VX -= 2 * approach * nx
			ball.VY -= 2 * approach * ny
			enemy.Destroyed = true
			w.emit(EnemyHit, enemy.X+enemy.W/2, enemy.Y+enemy.H/2, enemy.Kind, NoEffect)
		}
	}
}
