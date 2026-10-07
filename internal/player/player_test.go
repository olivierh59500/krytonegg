package player

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"

	"krytonegg/internal/game"
)

func fixture() *game.World {
	bricks := []game.Brick{}
	for row := 0; row < 2; row++ {
		for column := 0; column < 5; column++ {
			bricks = append(bricks, game.Brick{X: 112 + float64(column*16), Y: 56 + float64(row*8), W: 16, H: 8, Kind: 1, HP: 1, Destructible: true})
		}
	}
	return game.New([]game.Level{{Name: "INPUT-ONLY FIXTURE", Bricks: bricks}}, 42)
}

func cloneWorld(w *game.World) game.World {
	c := *w
	c.Balls = slices.Clone(w.Balls)
	c.Bricks = slices.Clone(w.Bricks)
	c.Drops = slices.Clone(w.Drops)
	c.Effects = slices.Clone(w.Effects)
	c.Enemies = slices.Clone(w.Enemies)
	c.Events = slices.Clone(w.Events)
	return c
}

func TestAgentNeverMutatesWorldAndReplayIsRepeatable(t *testing.T) {
	left, right := fixture(), fixture()
	a, b := New(Config{Seed: 42}), New(Config{Seed: 42})
	for tick := 0; tick < 12000; tick++ {
		before := cloneWorld(left)
		inA, inB := a.Next(left), b.Next(right)
		if !reflect.DeepEqual(before, *left) {
			t.Fatalf("agent mutated world at tick %d", tick)
		}
		if inA != inB {
			t.Fatalf("same seed produced different input at tick %d", tick)
		}
		left.Tick(inA)
		right.Tick(inB)
		if !reflect.DeepEqual(left, right) {
			t.Fatalf("replay diverged at tick %d", tick)
		}
		if left.State == game.Won {
			return
		}
		if left.State == game.GameOver {
			t.Fatalf("input-only fixture failed at tick %d", tick)
		}
	}
	t.Fatalf("expert failed to clear the input-only fixture: remaining %d, paddle %+v, balls %+v", left.RemainingBricks(), left.Paddle, left.Balls)
}

func TestHumanMouseMotionHasBoundedSpeedAndAcceleration(t *testing.T) {
	for _, speed := range []float64{7, 10, 12} {
		t.Run(fmt.Sprintf("speed_%g", speed), func(t *testing.T) {
			w := fixture()
			a := New(Config{Seed: 42, MaxSpeed: speed})
			previousX, previousVelocity := w.Paddle.X, 0.0
			for tick := 0; tick < 12000; tick++ {
				beforeState := w.State
				in := a.Next(w)
				if !in.UseMouse || in.Pause || in.Left || in.Right || in.Up || in.Down {
					t.Fatalf("unexpected input at tick %d: %+v", tick, in)
				}
				if math.IsNaN(in.PaddleX) || math.IsNaN(in.PaddleY) {
					t.Fatal("non-finite mouse input")
				}
				w.Tick(in)
				velocity := w.Paddle.X - previousX
				if math.Abs(velocity) > speed+1e-9 {
					t.Fatalf("paddle teleported at tick %d: %g", tick, velocity)
				}
				// A physical wall stops motion immediately; ordinary motion must brake.
				atWall := w.Paddle.X == game.FieldLeft+w.Paddle.W/2 || w.Paddle.X == game.FieldRight-1-w.Paddle.W/2
				if !atWall && (beforeState == game.Ready || beforeState == game.Playing) && math.Abs(velocity-previousVelocity) > 2+1e-9 {
					t.Fatalf("acceleration exceeded human constraint at tick %d: %g -> %g, state %d, input %+v, internal vx %g", tick, previousVelocity, velocity, beforeState, in, a.vx)
				}
				previousX, previousVelocity = w.Paddle.X, velocity
				if w.State == game.Won || w.State == game.GameOver {
					return
				}
			}
		})
	}
}

func TestOriginalBonusChoicesAvoidRecoveredPenalties(t *testing.T) {
	for _, id := range []uint8{4, 10, 11, 15, 19, 22, 23} {
		if bonusValue(game.BonusFromCode(id<<2)) >= 0 {
			t.Fatalf("original negative bonus %d was attractive", id)
		}
	}
	for _, id := range []uint8{1, 7, 8, 9, 12, 14, 16, 17, 18, 20, 21, 24, 25, 26, 27} {
		if bonusValue(game.BonusFromCode(id<<2)) <= 0 {
			t.Fatalf("original positive bonus %d was avoided", id)
		}
	}
}

func TestObservationsAreDelayedIndependentCopies(t *testing.T) {
	w := fixture()
	a := New(Config{ReactionTicks: 5})
	for tick := 0; tick < 10; tick++ {
		a.Next(w)
		if len(a.history) == 6 && a.history[0].tick != w.TickCount-5 {
			t.Fatal("reaction delay does not match PAL update count")
		}
		if len(a.history) > 0 {
			a.history[0].balls[0].X = -100
		}
		if w.Balls[0].X == -100 {
			t.Fatal("observation aliases real balls")
		}
		w.Tick(game.Input{})
	}
}
