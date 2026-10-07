package player

import (
	"krytonegg/internal/game"
	"math"
)

// Forecasts are private geometry estimates from an older visible frame. They do
// not execute a copied World and cannot create a bonus or alter the real ball.
func forecastLanding(ball game.Ball, o observation, paddleY float64, lag int) (landing, bool) {
	line := paddleY - o.paddle.H/2 - ball.Radius
	o.enemies = append([]game.Enemy(nil), o.enemies...)
	for tick := 0; tick < 420; tick++ {
		if tick >= lag && ball.VY > 0 && ball.Y >= line {
			return landing{ball: ball, x: ball.X, time: float64(tick - lag)}, true
		}
		if ball.Y-ball.Radius > game.FieldBottom {
			return landing{}, false
		}
		advancePrediction(&ball, o, nil, o.frame+uint64(tick+1))
		o.enemies, o.enemyTurn = advanceForecastEnemies(o.enemies, o.enemyTurn)
	}
	return landing{}, false
}

func forecastAim(ball game.Ball, o observation) float64 {
	// Long-range aiming treats moving creatures as uncertain rather than fixed walls.
	o.enemies = nil
	first := 0.0
	lowestY, targetX, targets := -1.0, 0.0, 0.0
	for _, b := range o.bricks {
		if b.Destroyed || !b.Destructible {
			continue
		}
		if b.Y > lowestY {
			lowestY, targetX, targets = b.Y, b.X+b.W/2, 1
		} else if b.Y == lowestY {
			targetX += b.X + b.W/2
			targets++
		}
	}
	if targets > 0 {
		targetX /= targets
	}
	// A broad permanent barrier can hide the remaining bricks. Move the rebound
	// lane toward its visible opening or temporary gate before aiming at the core.
	for row := game.LevelRows - 1; row >= 0; row-- {
		rowY := 24 + float64(row*8)
		if rowY < lowestY {
			continue
		}
		blocked := 0
		openX, openCount := 0.0, 0.0
		for column := 0; column < game.LevelColumns; column++ {
			cellX := game.FieldLeft + float64(column*16)
			var block *game.Brick
			for i := range o.bricks {
				b := &o.bricks[i]
				if b.X == cellX && b.Y == rowY && !b.Destroyed {
					block = b
					break
				}
			}
			permanent := block != nil && !block.Destructible && (block.Kind >= 0x61 && block.Kind <= 0x70 || block.Kind >= 0xf1 && block.Kind <= 0xf7)
			if permanent {
				blocked++
			} else {
				openX += cellX + 8
				openCount++
			}
		}
		if blocked >= game.LevelColumns/2 && openCount > 0 {
			targetX = openX / openCount
			break
		}
	}
	hit := make(map[int]bool)
	for tick := 0; tick < 190; tick++ {
		advancePrediction(&ball, o, func(index int) {
			if hit[index] {
				return
			}
			hit[index] = true
			b := o.bricks[index]
			if b.Destructible {
				first += 8/(1+float64(tick)*.012) + math.Max(0, bonusValue(b.Bonus))*.2
			} else if b.Kind >= 0x71 && b.Kind <= 0xe0 || b.Kind >= 0xe1 && b.Kind <= 0xfd {
				first += .5
			}
		}, o.frame+uint64(tick+1))
		if ball.VY > 0 && ball.Y > 178 {
			break
		}
	}
	if targets > 0 {
		first -= math.Abs(ball.X-targetX) * .012
	}
	return first
}

func advancePrediction(ball *game.Ball, o observation, onHit func(int), tick uint64) {
	steps := max(1, int(math.Ceil(math.Max(math.Abs(ball.VX), math.Abs(ball.VY))/.75)))
	for step := 0; step < steps; step++ {
		ball.X += ball.VX / float64(steps)
		ball.Y += ball.VY / float64(steps)
		if ball.X-ball.Radius < game.FieldLeft {
			ball.X = game.FieldLeft + ball.Radius
			ball.VX = math.Abs(ball.VX)
		}
		if ball.X+ball.Radius > game.FieldRight {
			ball.X = game.FieldRight - ball.Radius
			ball.VX = -math.Abs(ball.VX)
		}
		if ball.Y-ball.Radius < game.FieldTop {
			ball.Y = game.FieldTop + ball.Radius
			ball.VY = math.Abs(ball.VY)
		}
		if o.ghost {
			continue
		}
		for i, portal := range o.portals {
			_, _, _, touch := circleRect(ball.X, ball.Y, ball.Radius, portal.X-8, portal.Y-4, 16, 8)
			if !touch {
				continue
			}
			for j, target := range o.portals {
				if i != j && portal.Kind != target.Kind {
					ball.X = target.X - 8 + math.Copysign(16, ball.VX) + ball.Radius
					ball.Y = target.Y - 4 + math.Copysign(8, ball.VY) + ball.Radius
					break
				}
			}
			break
		}
		for i := range o.enemies {
			e := &o.enemies[i]
			if e.Destroyed {
				continue
			}
			_, _, _, touch := circleRect(ball.X, ball.Y, ball.Radius, e.X, e.Y, e.W, e.H)
			if !touch {
				continue
			}
			e.Destroyed, e.DeathTicks = true, 56
			if !o.super && !ball.Attached {
				ball.VY = -ball.VY
				ball.VX = math.Copysign(float64((tick&3)|1), ball.VX)
			}
		}
		for i, b := range o.bricks {
			if b.Destroyed || b.Kind == 0xf8 || b.Kind == 0xf9 {
				continue
			}
			if ball.X+ball.Radius < b.X || ball.X-ball.Radius > b.X+b.W || ball.Y+ball.Radius < b.Y || ball.Y-ball.Radius > b.Y+b.H {
				continue
			}
			nx, ny, depth, touch := brickContact(ball.X, ball.Y, ball.Radius, b.X, b.Y, b.W, b.H)
			if !touch {
				continue
			}
			if o.super {
				if onHit != nil {
					onHit(i)
				}
				continue
			}
			ball.X += nx * (depth + .001)
			ball.Y += ny * (depth + .001)
			approach := ball.VX*nx + ball.VY*ny
			if approach < 0 {
				if nx != 0 {
					ball.VX = -ball.VX
				} else {
					ball.VY = -ball.VY
				}
				if onHit != nil {
					onHit(i)
				}
			}
		}
	}
}

func circleRect(cx, cy, r, x, y, w, h float64) (nx, ny, depth float64, touch bool) {
	dx, dy := cx-clamp(cx, x, x+w), cy-clamp(cy, y, y+h)
	d2 := dx*dx + dy*dy
	if d2 > r*r {
		return 0, 0, 0, false
	}
	if d2 > 1e-12 {
		d := math.Sqrt(d2)
		return dx / d, dy / d, r - d, true
	}
	l, rr, t, b := cx-x, x+w-cx, cy-y, y+h-cy
	switch math.Min(math.Min(l, rr), math.Min(t, b)) {
	case l:
		return -1, 0, r + l, true
	case rr:
		return 1, 0, r + rr, true
	case t:
		return 0, -1, r + t, true
	default:
		return 0, 1, r + b, true
	}
}

func brickContact(cx, cy, r, x, y, width, height float64) (nx, ny, depth float64, hit bool) {
	nx, ny, _, hit = circleRect(cx, cy, r, x, y, width, height)
	if !hit {
		return 0, 0, 0, false
	}
	if math.Abs(nx) > math.Abs(ny) {
		if nx < 0 {
			return -1, 0, cx - (x - r), true
		}
		return 1, 0, x + width + r - cx, true
	}
	if ny < 0 {
		return 0, -1, cy - (y - r), true
	}
	return 0, 1, y + height + r - cy, true
}
