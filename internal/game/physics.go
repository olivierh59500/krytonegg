package game

import "math"

// updateBalls subdivides movement to less than one pixel per step, preventing
// even accelerated balls from crossing an eight-pixel brick between checks.
func (w *World) updateBalls() {
	live := w.Balls[:0]
	for _, source := range w.Balls {
		ball := source
		if !ball.Attached {
			steps := int(math.Ceil(math.Max(math.Abs(ball.VX), math.Abs(ball.VY)) / 0.75))
			if steps < 1 {
				steps = 1
			}
			hit := make(map[int]bool)
			for step := 0; step < steps; step++ {
				ball.X += ball.VX / float64(steps)
				ball.Y += ball.VY / float64(steps)
				w.collideWalls(&ball)
				w.collidePaddle(&ball)
				w.collidePortals(&ball)
				w.collideEnemies(&ball)
				if ball.Attached {
					break
				}
				if w.Active(GhostBall) {
					continue
				}
				for i := range w.Bricks {
					brick := &w.Bricks[i]
					if brick.Destroyed || brick.Kind == 0xf8 || brick.Kind == 0xf9 {
						continue
					}
					nx, ny, depth, collision := circleRect(ball.X, ball.Y, ball.Radius, brick.X, brick.Y, brick.W, brick.H)
					if !collision {
						continue
					}
					if w.Active(SuperBall) && brick.Destructible {
						if !hit[i] {
							w.damageBrick(i)
							hit[i] = true
						}
						continue
					}
					ball.X += nx * (depth + 0.001)
					ball.Y += ny * (depth + 0.001)
					approach := ball.VX*nx + ball.VY*ny
					if approach < 0 {
						ball.VX -= 2 * approach * nx
						ball.VY -= 2 * approach * ny
						if !hit[i] {
							w.damageBrick(i)
							hit[i] = true
						}
					}
				}
				if ball.Y-ball.Radius > FieldBottom {
					break
				}
			}
		}
		if ball.Y-ball.Radius > FieldBottom {
			w.emit(BallLost, ball.X, ball.Y, ball.ID, NoEffect)
		} else {
			live = append(live, ball)
		}
	}
	w.Balls = live
}

func (w *World) collidePortals(ball *Ball) {
	for i, source := range w.Portals {
		_, _, _, collision := circleRect(ball.X, ball.Y, ball.Radius, source.X-8, source.Y-4, 16, 8)
		if !collision {
			continue
		}
		for j, target := range w.Portals {
			if i != j && source.Kind != target.Kind {
				ball.X = target.X - 8 + math.Copysign(16, ball.VX) + ball.Radius
				ball.Y = target.Y - 4 + math.Copysign(8, ball.VY) + ball.Radius
				return
			}
		}
	}
}

func (w *World) collideWalls(ball *Ball) {
	bounced := false
	if ball.X-ball.Radius < FieldLeft {
		ball.X = FieldLeft + ball.Radius
		if ball.VX < 0 {
			ball.VX = -ball.VX
			bounced = true
		}
	}
	if ball.X+ball.Radius > FieldRight {
		ball.X = FieldRight - ball.Radius
		if ball.VX > 0 {
			ball.VX = -ball.VX
			bounced = true
		}
	}
	if ball.Y-ball.Radius < FieldTop {
		ball.Y = FieldTop + ball.Radius
		if ball.VY < 0 {
			ball.VY = -ball.VY
			bounced = true
		}
	}
	if bounced {
		w.emit(Bounce, ball.X, ball.Y, 0, NoEffect)
	}
}

func (w *World) collidePaddle(ball *Ball) {
	if w.SecondPaddle != nil {
		w.collidePaddleShape(ball, *w.SecondPaddle, false)
	}
	w.collidePaddleShape(ball, w.Paddle, true)
}

func (w *World) collidePaddleShape(ball *Ball, paddle Paddle, main bool) {
	// Catch only the upper paddle surface; moving above a ball cannot rescue it.
	if ball.VY <= 0 || ball.Y > paddle.Y {
		return
	}
	_, _, _, collision := circleRect(ball.X, ball.Y, ball.Radius,
		paddle.X-paddle.W/2, paddle.Y-paddle.H/2, paddle.W, paddle.H)
	if !collision {
		return
	}
	ball.Y = paddle.Y - paddle.H/2 - ball.Radius - 0.01
	if main && w.Active(Sticky) {
		ball.Attached = true
		ball.AttachedOffset = clamp(ball.X-paddle.X, -paddle.W/2+ball.Radius, paddle.W/2-ball.Radius)
		ball.VX, ball.VY = 0, 0
	} else {
		left, right := paddle.X-paddle.W/2, paddle.X+paddle.W/2
		if ball.X < left+4 && ball.VX > 0 || ball.X > right-4 && ball.VX < 0 {
			ball.VX = -ball.VX
		}
		ball.VY = -ball.VY
		if main {
			movement := int(left) - int(w.previousPaddleLeft)
			ball.VX = clamp(ball.VX+float64(movement>>1), -3, 3)
		}
	}
	w.emit(Bounce, ball.X, ball.Y, 1, NoEffect)
}

// circleRect returns the outward contact normal and the required separation.
// Rounded corner contacts use a radial normal instead of an axis-only bounce.
func circleRect(cx, cy, radius, x, y, width, height float64) (nx, ny, depth float64, hit bool) {
	closestX, closestY := clamp(cx, x, x+width), clamp(cy, y, y+height)
	dx, dy := cx-closestX, cy-closestY
	distanceSquared := dx*dx + dy*dy
	if distanceSquared > radius*radius {
		return 0, 0, 0, false
	}
	if distanceSquared > 1e-12 {
		distance := math.Sqrt(distanceSquared)
		return dx / distance, dy / distance, radius - distance, true
	}
	// A moving paddle or externally restored replay may place a center inside a
	// rectangle. Resolve to the nearest face rather than creating a NaN normal.
	left, right, top, bottom := cx-x, x+width-cx, cy-y, y+height-cy
	nearest := math.Min(math.Min(left, right), math.Min(top, bottom))
	switch nearest {
	case left:
		return -1, 0, radius + left, true
	case right:
		return 1, 0, radius + right, true
	case top:
		return 0, -1, radius + top, true
	default:
		return 0, 1, radius + bottom, true
	}
}

func (w *World) damageBrick(index int) {
	brick := &w.Bricks[index]
	if brick.Destroyed {
		return
	}
	if brick.Kind >= 0xf5 && brick.Kind <= 0xf7 {
		w.resizePaddle(-2 * (brick.Kind - 0xf4))
		w.emit(Bounce, brick.X+brick.W/2, brick.Y+brick.H/2, 2, NoEffect)
		return
	}
	if brick.Kind >= 0x71 && brick.Kind <= 0xe0 {
		brick.HitCount++
		if brick.HitCount >= 6 {
			brick.Destroyed, brick.TemporaryTimer = true, 251
			brick.HitCount = 0
		}
		w.emit(BrickHit, brick.X+brick.W/2, brick.Y+brick.H/2, brick.HitCount, NoEffect)
		return
	}
	if brick.Kind >= 0xfa && brick.Kind <= 0xfd {
		brick.Destroyed, brick.TemporaryTimer = true, ((brick.Kind-0xf9)<<5)+1
		w.emit(BrickHit, brick.X+brick.W/2, brick.Y+brick.H/2, 0, NoEffect)
		return
	}
	if !brick.Destructible {
		if brick.Kind >= 0x61 && brick.Kind <= 0x70 {
			brick.FlashTimer = 8
		}
		if brick.Kind >= 0xe1 && brick.Kind <= 0xf0 {
			brick.Destroyed, brick.TemporaryTimer = true, 163
		}
		w.emit(Bounce, brick.X+brick.W/2, brick.Y+brick.H/2, 2, NoEffect)
		return
	}
	brick.HP--
	w.emit(BrickHit, brick.X+brick.W/2, brick.Y+brick.H/2, brick.HP, NoEffect)
	if brick.HP > 0 {
		if brick.Kind >= 0x11 && brick.Kind <= 0x30 || brick.Kind >= 0x41 && brick.Kind <= 0x60 {
			brick.Kind -= 0x10
		}
		return
	}
	brick.Destroyed = true
	// Native scoring uses a 16-bit accumulator and a six-bit 68000 shift count.
	award := uint16(uint64(brick.Score) << uint(w.ScoreMultiplier&63))
	w.Score = int(uint16(w.Score) + award)
	w.emit(BrickBreak, brick.X+brick.W/2, brick.Y+brick.H/2, brick.Kind, brick.Bonus)
	rawKind := int(brick.Code & 0xff)
	if brick.Bonus != NoEffect || rawKind >= 0x31 && rawKind <= 0x60 {
		drop := Drop{X: brick.X + brick.W/2, Y: brick.Y + brick.H/2, VY: 1, Kind: brick.Bonus, Code: uint8(brick.Code >> 8), Power: brick.BonusPower, Tile: brick.Kind}
		w.Drops = append(w.Drops, drop)
		w.emit(BonusSpawn, drop.X, drop.Y, int(drop.Code), drop.Kind)
	}
}

func (w *World) updateBricks() {
	for i := range w.Bricks {
		brick := &w.Bricks[i]
		if brick.FlashTimer > 0 {
			brick.FlashTimer--
		}
		if brick.TemporaryTimer <= 0 {
			continue
		}
		if (brick.Kind < 0xe1 || brick.Kind > 0xf0) && w.TickCount%4 != 0 {
			continue
		}
		brick.TemporaryTimer--
		if brick.TemporaryTimer == 0 {
			brick.Destroyed = false
			brick.Kind = brick.OriginalKind
		}
	}
}

func (w *World) updateDrops() {
	live := w.Drops[:0]
	for _, drop := range w.Drops {
		drop.Y += drop.VY
		_, _, _, caught := circleRect(drop.X, drop.Y, 5,
			w.Paddle.X-w.Paddle.W/2, w.Paddle.Y-w.Paddle.H/2, w.Paddle.W, w.Paddle.H)
		if caught {
			w.ApplyBonusPower(drop.Kind, drop.Power)
			w.emit(BonusCaught, drop.X, drop.Y, int(drop.Code), drop.Kind)
			continue
		}
		if drop.Y < FieldBottom+8 {
			live = append(live, drop)
		}
	}
	w.Drops = live
}

func (w *World) fire() {
	if len(w.Shots) >= 5 {
		return
	}
	rapid := w.Active(EnhancedCannon) || w.WeaponMode&(4|8) != 0
	if len(w.Shots) > 0 && !rapid {
		return
	}
	rocket := w.Active(Rocket)
	cannon := w.Active(Laser) || w.Active(EnhancedCannon)
	velocity := -4.0
	wide := w.WeaponMode&(2|8) != 0 || cannon
	if w.WeaponMode&(4|8) != 0 {
		velocity = -8
	}
	width := 1.0
	if wide {
		width = 13
	}
	w.Shots = append(w.Shots, Shot{X: w.Paddle.X, Y: w.Paddle.Y - w.Paddle.H/2, VY: velocity, Rocket: rocket, Cannon: cannon, Width: width})
	w.emit(ShotFired, w.Paddle.X, w.Paddle.Y, 0, NoEffect)
}

func (w *World) updateShots() {
	live := w.Shots[:0]
	for _, shot := range w.Shots {
		shot.Y += shot.VY
		if shot.Y < FieldTop {
			continue
		}
		hitEnemy := false
		for i := range w.Enemies {
			enemy := &w.Enemies[i]
			if !enemy.Destroyed && shot.X+shot.Width/2 >= enemy.X && shot.X-shot.Width/2 <= enemy.X+enemy.W && shot.Y <= enemy.Y+enemy.H && shot.Y-shot.VY >= enemy.Y {
				enemy.Destroyed, hitEnemy = true, true
				w.emit(EnemyHit, enemy.X+enemy.W/2, enemy.Y+enemy.H/2, enemy.Kind, NoEffect)
				break
			}
		}
		if hitEnemy {
			continue
		}
		hit := -1
		for i, brick := range w.Bricks {
			if !brick.Destroyed && shot.X+shot.Width/2 >= brick.X && shot.X-shot.Width/2 <= brick.X+brick.W && shot.Y <= brick.Y+brick.H && shot.Y-shot.VY >= brick.Y {
				if hit < 0 || brick.Y > w.Bricks[hit].Y {
					hit = i
				}
			}
		}
		if hit < 0 {
			live = append(live, shot)
			continue
		}
		if shot.Cannon && w.Bricks[hit].Destructible {
			w.Bricks[hit].HP = 1
		}
		w.damageBrick(hit)
		if shot.Rocket {
			centerX, centerY := w.Bricks[hit].X+w.Bricks[hit].W/2, w.Bricks[hit].Y+w.Bricks[hit].H/2
			for i, brick := range w.Bricks {
				if i != hit && !brick.Destroyed && math.Hypot(brick.X+brick.W/2-centerX, brick.Y+brick.H/2-centerY) < 20 {
					w.damageBrick(i)
				}
			}
		}
	}
	w.Shots = live
}
