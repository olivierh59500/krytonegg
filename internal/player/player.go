// Package player supplies a deterministic, input-only expert player for review.
// Its reaction delay and bounded mouse motion deliberately remain visible.
package player

import (
	"math"
	"sort"

	"krytonegg/internal/game"
)

// Config specifies human control constraints in native pixels and PAL updates.
// Zero values select a 100 ms reaction, 350 px/s speed, and 2 px/update² acceleration.
type Config struct {
	Seed                   uint64
	ReactionTicks          int
	MaxSpeed, Acceleration float64
	LaunchDelay            int
}

type observation struct {
	state                                game.State
	level                                int
	tick                                 uint64
	frame                                uint64
	paddle                               game.Paddle
	balls                                []game.Ball
	bricks                               []game.Brick
	drops                                []game.Drop
	enemies                              []game.Enemy
	portals                              []game.Portal
	events                               []game.Event
	super, ghost, flying, weapon, cannon bool
	shield                               int
	enemyTurn                            int
	combat                               *game.CombatData
	combatYRate                          float64
}

// Agent converts delayed observations into ordinary game.Input values.
type Agent struct {
	config             Config
	history            []observation
	frames             int
	state              game.State
	level              int
	stateFrames        int
	readyAttempt       int
	vx, vy             float64
	pointerX, pointerY float64
	pointerSeen        bool
	seed               uint64
	aimBall            int
	aimVX              float64
	aimValid           bool
	lastBounceTick     uint64
	lastRemaining      int
	stagnant           int
	combatMotion       []float64
}

// New constructs a reproducible player without changing campaign rules.
func New(config Config) *Agent {
	if config.Seed == 0 {
		config.Seed = 1990
	}
	if config.ReactionTicks <= 0 {
		config.ReactionTicks = 5
	}
	if config.MaxSpeed <= 0 {
		config.MaxSpeed = 7
	}
	if config.Acceleration <= 0 {
		config.Acceleration = 2
	}
	if config.LaunchDelay <= 0 {
		config.LaunchDelay = 25
	}
	return &Agent{config: config, level: -1, seed: config.Seed}
}

// Next observes the world and returns a legal mouse/buttons input. It never calls
// Tick, ApplyBonus, StartAt, or writes to a world, ball, brick, counter, or effect.
func (a *Agent) Next(w *game.World) game.Input {
	if w == nil {
		return game.Input{}
	}
	a.frames++
	if w.State != a.state || w.LevelIndex != a.level {
		if w.LevelIndex != a.level {
			a.readyAttempt = 0
		} else if w.State == game.Ready && a.state == game.Dying {
			a.readyAttempt++
		}
		a.state, a.level, a.stateFrames = w.State, w.LevelIndex, 0
		a.aimValid = false
		if w.State == game.Ready || w.State == game.Combat {
			a.history = nil
			a.vx, a.vy = 0, 0
			if w.State == game.Combat {
				a.combatMotion = nil
			}
		}
	}
	a.stateFrames++
	observed := observe(w)
	observed.combatYRate = 1
	if observed.combat != nil && len(a.history) > 0 && a.history[len(a.history)-1].combat != nil {
		previous := a.history[len(a.history)-1].combat
		numerator, denominator := 0.0, 0.0
		for _, shot := range observed.combat.BossShots {
			for _, old := range previous.BossShots {
				if math.Abs(shot.X-old.X-old.VX) < .01 && math.Abs(old.VY) > .01 {
					numerator += math.Abs(shot.Y - old.Y)
					denominator += math.Abs(old.VY)
					break
				}
			}
		}
		if denominator > 0 {
			a.combatMotion = append(a.combatMotion, clamp(numerator/denominator, 0, 1))
			if len(a.combatMotion) > 8 {
				a.combatMotion = a.combatMotion[1:]
			}
		}
		if len(a.combatMotion) > 0 {
			observed.combatYRate = 0
			for _, ratio := range a.combatMotion {
				observed.combatYRate += ratio / float64(len(a.combatMotion))
			}
		}
	}
	a.history = append(a.history, observed)
	if len(a.history) > a.config.ReactionTicks+1 {
		a.history = a.history[1:]
	}
	o := a.history[0]
	lag := len(a.history) - 1
	for _, event := range o.events {
		if event.Kind == game.Bounce && event.Value == 1 && o.tick != a.lastBounceTick {
			a.aimValid = false
			a.lastBounceTick = o.tick
		}
	}
	x, y := w.Paddle.X, w.Paddle.Y
	targetX, targetY := x, y
	in := game.Input{UseMouse: true}
	switch w.State {
	case game.Title:
		in.Launch = a.stateFrames == a.config.LaunchDelay
	case game.Ready:
		// Reposition before launch rather than firing immediately after a loss.
		targetX = game.FieldLeft + 36 + float64((a.level*43+a.readyAttempt*73)%210)
		in.Launch = a.stateFrames == a.config.LaunchDelay+25
	case game.LevelClear:
		in.Launch = a.stateFrames == 40
	case game.Playing:
		remaining := w.RemainingBricks()
		if remaining != a.lastRemaining {
			a.stagnant = 0
			a.lastRemaining = remaining
		} else {
			a.stagnant++
		}
		targetX, targetY = a.roundTarget(o, lag, w.Paddle)
		in.Fire = true
		for _, ball := range o.balls {
			if ball.Attached && a.stateFrames%20 == 0 {
				in.Launch = true
			}
		}
	case game.Combat:
		if o.combat != nil {
			x, y = w.Combat.ShipX, w.Combat.ShipY
			targetX, targetY = x, a.combatTarget(o.combat, lag, y, o.combatYRate)
			in.Fire = true
		}
	}
	dx := a.moveAxis(targetX-x, &a.vx)
	dy := a.moveAxis(targetY-y, &a.vy)
	if w.State == game.Combat {
		dy = targetY - y
		a.vy = dy
	}
	if w.State != game.Combat && !w.Active(game.Flying) {
		dy, a.vy = 0, 0
	}
	if w.State == game.Combat {
		in.PaddleX, in.PaddleY = x+dx, clamp(y+dy, 0, 134)
		a.pointerX, a.pointerY, a.pointerSeen = in.PaddleX, in.PaddleY, false
		return in
	}
	// Convert physical motion to mouse deltas when native control modifiers apply.
	if !a.pointerSeen {
		a.pointerX, a.pointerY, a.pointerSeen = x, y, true
	}
	factor := 1.0
	if w.Active(game.MouseSensitivity) {
		factor = 3
	}
	if w.Active(game.Reverse) {
		dx = -dx
	}
	if w.Active(game.MouseSensitivity) || w.Active(game.Reverse) {
		a.pointerX += dx / factor
		a.pointerY += dy / factor
	} else {
		a.pointerX, a.pointerY = x+dx, y+dy
	}
	in.PaddleX, in.PaddleY = a.pointerX, a.pointerY
	return in
}

func (a *Agent) moveAxis(distance float64, velocity *float64) float64 {
	// Braking before the target prevents perfect instantaneous direction changes.
	previous := *velocity
	desired := math.Copysign(math.Min(a.config.MaxSpeed, math.Sqrt(2*a.config.Acceleration*math.Abs(distance))), distance)
	if math.Abs(distance) < .05 {
		desired = 0
	}
	*velocity += clamp(desired-*velocity, -a.config.Acceleration, a.config.Acceleration)
	if math.Abs(*velocity) > math.Abs(distance) && *velocity*distance >= 0 && math.Abs(previous-distance) <= a.config.Acceleration {
		*velocity = distance
	}
	return *velocity
}

func observe(w *game.World) observation {
	o := observation{state: w.State, level: w.LevelIndex, tick: w.TickCount, frame: w.FrameCounter, paddle: w.Paddle,
		balls: append([]game.Ball(nil), w.Balls...), bricks: append([]game.Brick(nil), w.Bricks...),
		drops: append([]game.Drop(nil), w.Drops...), enemies: append([]game.Enemy(nil), w.Enemies...),
		portals: append([]game.Portal(nil), w.Portals...), super: w.Active(game.SuperBall), ghost: w.Active(game.GhostBall),
		events: append([]game.Event(nil), w.Events...),
		flying: w.Active(game.Flying), weapon: w.Active(game.Laser) || w.Active(game.Rocket) || w.Active(game.EnhancedCannon) || w.WeaponMode != 0}
	o.shield = w.ShieldCharges
	o.cannon = w.Active(game.Laser) || w.Active(game.EnhancedCannon)
	o.enemyTurn = w.EnemyTurnCounter()
	if w.Combat != nil {
		c := *w.Combat
		c.BossShots = append([]game.CombatShot(nil), c.BossShots...)
		o.combat = &c
	}
	return o
}

type landing struct {
	ball    game.Ball
	x, time float64
}

func (a *Agent) roundTarget(o observation, lag int, paddle game.Paddle) (float64, float64) {
	y := paddle.Y
	if o.flying {
		lowestBrick, lowestBall := 32.0, 0.0
		for _, brick := range o.bricks {
			if brick.Destructible && !brick.Destroyed && brick.Y+brick.H > lowestBrick {
				lowestBrick = brick.Y + brick.H
			}
		}
		for _, ball := range o.balls {
			if ball.Y > lowestBall {
				lowestBall = ball.Y
			}
		}
		y = clamp(lowestBrick+16, 80, 173)
		if lowestBall > y-paddle.H/2-12 {
			y = math.Max(paddle.Y, clamp(lowestBall+24, 80, 189))
		}
		if a.stagnant > 500 && lowestBall < paddle.Y-16 {
			// A flying ship can intercept a recurring orbit above an obstacle rather
			// than waiting indefinitely for a ball to return below the brick field.
			y = clamp(lowestBall+24, 36, 189)
		}
	}
	for _, ball := range o.balls {
		if ball.Attached {
			y = 189
		}
	}
	best := landing{time: math.Inf(1)}
	for _, ball := range o.balls {
		if ball.Attached {
			continue
		}
		candidate, ok := forecastLanding(ball, o, y, lag)
		if !ok {
			continue
		}
		travel := math.Abs(candidate.x-paddle.X) / a.config.MaxSpeed
		if travel > candidate.time+float64(paddle.W/2)/a.config.MaxSpeed {
			continue
		}
		if candidate.time < best.time {
			best = candidate
		}
	}
	x := paddle.X
	if !math.IsInf(best.time, 1) {
		x = best.x
		if a.aimBall != best.ball.ID || !a.aimValid || (best.time > 30 && a.frames%25 == 0) {
			a.aimVX = a.chooseAim(best, o, y)
			a.aimBall, a.aimValid = best.ball.ID, true
		}
		// Native rebounds respond to movement on contact, not position within a
		// conventional angled paddle. Prepare a short stroke toward the chosen lane.
		kick := clamp(2*(a.aimVX-best.ball.VX), -6, 6)
		if best.time > 3 && best.time < 20 {
			x = best.x - kick*2
		}
		if best.time <= 3 {
			x = paddle.X + kick
		}
		// Keep the ball safely inside the collision surface while applying momentum.
		x = clamp(x, best.x-paddle.W/2+5, best.x+paddle.W/2-5)
	}
	// Collect valuable drops only when their intercept does not sacrifice a ball.
	valueBest := 0.0
	for _, drop := range o.drops {
		t := (y-paddle.H/2-5-drop.Y)/math.Max(.1, drop.VY) - float64(lag)
		if t < 0 || t > 70 {
			continue
		}
		value := dropValue(drop.Kind, o)
		for _, ball := range o.balls {
			if !ball.Attached && math.Abs(ball.VY) < .8 && (drop.Kind == game.Fast || drop.Kind == game.Slow) {
				value = 12
			}
		}
		if value <= 0 {
			continue
		}
		travel := math.Abs(drop.X-paddle.X) / a.config.MaxSpeed
		if travel > t+4 {
			continue
		}
		if best.time < t+math.Abs(drop.X-best.x)/a.config.MaxSpeed+12 {
			continue
		}
		value /= 1 + t*.035 + math.Abs(drop.X-paddle.X)*.006
		if value > valueBest {
			valueBest, x = value, drop.X
		}
	}
	if o.weapon && best.time > 32 && valueBest == 0 {
		closest, distance := x, math.Inf(1)
		for _, b := range o.bricks {
			if b.Destroyed || !b.Destructible {
				continue
			}
			bx := b.X + b.W/2
			blocked := false
			for _, barrier := range o.bricks {
				if !barrier.Destroyed && !barrier.Destructible && barrier.Y > b.Y && bx >= barrier.X && bx <= barrier.X+barrier.W {
					blocked = true
					break
				}
			}
			if (!blocked || o.cannon) && math.Abs(bx-paddle.X) < distance {
				closest, distance = bx, math.Abs(bx-paddle.X)
			}
		}
		if best.time > math.Abs(closest-best.x)/a.config.MaxSpeed+20 {
			x = closest
		}
	}
	// Negative drops and lethal original enemy flags are visible hazards.
	for _, drop := range o.drops {
		if dropValue(drop.Kind, o) >= 0 || drop.Y+float64(lag)*drop.VY < y-18 {
			continue
		}
		if math.Abs(x-drop.X) < paddle.W/2+7 && best.time > 8 && (valueBest == 0 || drop.Kind == game.LargeBall && !o.cannon) {
			direction := math.Copysign(1, paddle.X-drop.X)
			x = drop.X + direction*(paddle.W/2+10)
		}
	}
	x = a.avoidEnemies(x, y, paddle, o, lag, best.time)
	return clamp(x, game.FieldLeft+paddle.W/2, game.FieldRight-1-paddle.W/2), y
}

func (a *Agent) avoidEnemies(target, y float64, p game.Paddle, o observation, lag int, landingTime float64) float64 {
	if o.shield > 0 && landingTime < 10 {
		return target
	}
	lo, hi := game.FieldLeft+p.W/2, game.FieldRight-1-p.W/2
	best, bestCost := clamp(target, lo, hi), math.Inf(1)
	candidates := []float64{target, p.X, p.X - 20, p.X + 20, p.X - 40, p.X + 40, lo, hi}
	future := append([]game.Enemy(nil), o.enemies...)
	counter := o.enemyTurn
	frames := make([][]game.Enemy, lag+45)
	for frame := range frames {
		future, counter = advanceForecastEnemies(future, counter)
		frames[frame] = append([]game.Enemy(nil), future...)
	}
	for _, e := range o.enemies {
		if e.Code&0x1000 != 0 {
			candidates = append(candidates, e.X-p.W/2-12, e.X+e.W+p.W/2+12)
		}
	}
	for _, candidate := range candidates {
		candidate = clamp(candidate, lo, hi)
		cost := math.Abs(candidate-target) * .02
		if landingTime < 20 {
			cost *= 3
		}
		px, py, vx, vy := p.X, p.Y, a.vx, a.vy
		for frame, creatures := range frames {
			if frame < lag {
				continue
			}
			px = clamp(px+a.moveAxis(candidate-px, &vx), lo, hi)
			py = clamp(py+a.moveAxis(y-py, &vy), 36, 189)
			for _, e := range creatures {
				if e.Destroyed || e.Code&0x1000 == 0 {
					continue
				}
				horizon := float64(frame - lag + 1)
				if e.Y+e.H < py-p.H/2-3 || e.Y > py+p.H/2+3 {
					continue
				}
				if px+p.W/2 > e.X-5 && px-p.W/2 < e.X+e.W+5 {
					cost += 5 / (1 + horizon*.08)
				}
			}
		}
		if cost < bestCost {
			best, bestCost = candidate, cost
		}
	}
	return best
}

func advanceForecastEnemies(enemies []game.Enemy, counter int) ([]game.Enemy, int) {
	live := enemies[:0]
	for _, enemy := range enemies {
		if enemy.Destroyed && enemy.DeathTicks <= 0 {
			continue
		}
		enemy, counter = game.AdvanceEnemyKinematics(enemy, counter)
		if enemy.Destroyed {
			enemy.DeathTicks--
			if enemy.DeathTicks <= 0 {
				continue
			}
		}
		live = append(live, enemy)
	}
	return live, counter
}

// bonusValue uses the recovered dispatch identities, never an assumed sprite order.
func bonusValue(kind game.EffectKind) float64 {
	switch kind {
	case game.ExtraLife:
		return 12
	case game.Laser, game.EnhancedCannon, game.WeaponMode1, game.WeaponMode2, game.WeaponMode4, game.WeaponMode8:
		return 10
	case game.Grow, game.Shield, game.SuperBall:
		return 8
	case game.Multiball, game.LargeBall:
		return 6
	case game.Slow, game.SecondPaddle:
		return 4
	case game.Flying, game.ScoreBonus:
		return 2
	case game.Reverse, game.Shrink, game.Darkness, game.GhostPaddle, game.GhostBall, game.Fast, game.RandomBonus:
		return -5
	case game.Autopilot, game.MouseSensitivity, game.Sticky:
		return -1
	default:
		return 0
	}
}

func dropValue(kind game.EffectKind, o observation) float64 {
	if kind == game.LargeBall {
		return -6
	}
	return bonusValue(kind)
}

func (a *Agent) chooseAim(l landing, o observation, y float64) float64 {
	bestVX, bestScore := l.ball.VX, math.Inf(-1)
	for _, increment := range []float64{-3, -2, -1, 0, 1, 2, 3} {
		vx := clamp(l.ball.VX+increment, -3, 3)
		ball := l.ball
		ball.X, ball.Y, ball.VX, ball.VY = l.x, y-o.paddle.H/2-ball.Radius-.05, vx, -math.Abs(ball.VY)
		if ball.VY == 0 {
			ball.VY = -2
		}
		score := forecastAim(ball, o)
		// A small reproducible variation prevents recurring identical lanes.
		phase := float64(int64(a.seed)-1990) * .017
		score += .04 * math.Sin(float64(a.frames/50)+vx*1.7+phase)
		if vx == 0 {
			score -= .15 + math.Min(5, float64(a.stagnant)/1000)
		}
		if a.stagnant > 1500 {
			score += .4 * math.Sin(float64(a.frames/300)+vx*2.1)
		}
		if score > bestScore {
			bestScore, bestVX = score, vx
		}
	}
	return bestVX
}

func (a *Agent) combatTarget(c *game.CombatData, lag int, currentY float64, yRate float64) float64 {
	type route struct {
		y, v, first, cost float64
		hit               uint64
	}
	beam := []route{{y: currentY, v: a.vy, first: currentY}}
	for frame := 1; frame <= 45; frame++ {
		next := make([]route, 0, len(beam)*4)
		seen := map[[3]int]float64{}
		for _, path := range beam {
			for _, accel := range []float64{-a.config.Acceleration, 0, a.config.Acceleration} {
				v := clamp(path.v+accel, -a.config.MaxSpeed, a.config.MaxSpeed)
				y := clamp(path.y+v, 0, 134)
				v = y - path.y
				candidate := route{y: y, v: v, first: path.first, cost: path.cost + math.Abs(accel)*.001, hit: path.hit}
				if frame == 1 {
					candidate.first = y
				}
				if y >= 56 && y <= 79 {
					candidate.cost -= .05
				} else {
					candidate.cost += math.Min(math.Abs(y-56), math.Abs(y-79)) * .0003
				}
				for index, shot := range c.BossShots {
					if index >= 64 || candidate.hit&(uint64(1)<<index) != 0 {
						continue
					}
					steps := lag + frame
					sx := shot.X + shot.VX*float64(steps)
					if sx <= 4 || sx > 32 {
						continue
					}
					verticalSteps := float64(steps) * yRate
					if yRate > .35 && yRate < .65 {
						verticalSteps = float64((c.Tick+steps)/2 - c.Tick/2)
					}
					sy := shot.Y + shot.VY*verticalSteps
					if sy+12 >= y && sy <= y+32 {
						candidate.cost += 100
						candidate.hit |= uint64(1) << index
					} else if sy+13 >= y && sy <= y+33 {
						candidate.cost += 1
					}
				}
				key := [3]int{int(math.Round(y / 2)), int(math.Round(v)), int(candidate.hit)}
				if old, ok := seen[key]; ok && old <= candidate.cost {
					continue
				}
				seen[key] = candidate.cost
				next = append(next, candidate)
			}
		}
		sort.SliceStable(next, func(i, j int) bool { return next[i].cost < next[j].cost })
		if len(next) > 160 {
			next = next[:160]
		}
		beam = next
	}
	return beam[0].first
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(v, hi)) }
