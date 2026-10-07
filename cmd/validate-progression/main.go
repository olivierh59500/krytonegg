// validate-progression runs the real game rules faster than real time using
// exclusively bounded expert-player inputs. Practice checks are labelled apart.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"krytonegg/assets"
	"krytonegg/internal/game"
	"krytonegg/internal/player"
)

type stage struct {
	Kind         string   `json:"kind"`
	Number       int      `json:"number"`
	Ticks        uint64   `json:"ticks"`
	Seconds      float64  `json:"seconds"`
	LivesStart   int      `json:"lives_start"`
	LivesEnd     int      `json:"lives_end"`
	Remaining    int      `json:"remaining"`
	Score        int      `json:"score"`
	Outcome      string   `json:"outcome"`
	PlayerEnergy int      `json:"player_energy,omitempty"`
	BossEnergy   int      `json:"boss_energy,omitempty"`
	Failure      *failure `json:"failure,omitempty"`
}
type failure struct {
	Paddle  game.Paddle      `json:"paddle"`
	Balls   []game.Ball      `json:"balls"`
	Bricks  []game.Brick     `json:"live_bricks"`
	Effects []game.Effect    `json:"effects"`
	Enemies []game.Enemy     `json:"enemies"`
	Combat  *game.CombatData `json:"combat,omitempty"`
}
type report struct {
	Mode               string  `json:"mode"`
	Seed               uint64  `json:"seed"`
	ReactionMS         int     `json:"reaction_ms"`
	MaxPixelsPerSecond float64 `json:"max_pixels_per_second"`
	Acceleration       float64 `json:"pixels_per_tick_squared"`
	CheatsEnabled      bool    `json:"cheats_enabled"`
	Stages             []stage `json:"stages"`
	Completed          bool    `json:"completed"`
	WallSeconds        float64 `json:"wall_seconds"`
}

func main() {
	seed := flag.Uint64("seed", 1990, "deterministic simulation and player seed")
	mode := flag.String("mode", "campaign", "campaign, rounds, or combats; practice modes start each authentic stage separately")
	first := flag.Int("first", 1, "first practice round or combat")
	last := flag.Int("last", 60, "last practice round; combats always stop at six")
	limit := flag.Uint64("ticks", 180000, "maximum simulation updates per stage")
	reaction := flag.Int("reaction", 5, "reaction delay in PAL updates")
	speed := flag.Float64("speed", 7, "maximum native pixels per update")
	acceleration := flag.Float64("acceleration", 2, "maximum native pixels per update squared")
	output := flag.String("out", "", "write the final JSON report to this file")
	tracePath := flag.String("trace", "", "write every submitted input and the observed state as JSONL")
	flag.Parse()
	if *mode != "campaign" && *mode != "rounds" && *mode != "combats" {
		fatal(fmt.Errorf("unknown mode %q", *mode))
	}
	if *limit == 0 || *reaction < 1 || *speed <= 0 || *acceleration <= 0 {
		fatal(fmt.Errorf("ticks, reaction, and speed must be positive"))
	}
	if *mode == "rounds" && (*first < 1 || *last < *first || *last > 60) {
		fatal(fmt.Errorf("practice rounds must be an ordered selection within 1 through 60"))
	}
	if *mode == "combats" && (*first < 1 || *first > 6) {
		fatal(fmt.Errorf("first practice combat must be within 1 through 6"))
	}
	table, err := assets.Read("original/level.tab")
	if err != nil {
		fatal(err)
	}
	meta, err := assets.Read("levels.json")
	if err != nil {
		fatal(err)
	}
	levels, err := game.LoadCampaign(table, meta)
	if err != nil {
		fatal(err)
	}
	combatData, err := assets.Read("combat.json")
	if err != nil {
		fatal(err)
	}
	var combat struct {
		Boundary []int `json:"boss_boundary_by_4px_row"`
	}
	if err = json.Unmarshal(combatData, &combat); err != nil {
		fatal(err)
	}
	var trace *json.Encoder
	if *tracePath != "" {
		f, err := os.Create(*tracePath)
		if err != nil {
			fatal(err)
		}
		defer f.Close()
		trace = json.NewEncoder(f)
	}
	config := player.Config{Seed: *seed, ReactionTicks: *reaction, MaxSpeed: *speed, Acceleration: *acceleration}
	r := report{Mode: *mode, Seed: *seed, ReactionMS: *reaction * 20, MaxPixelsPerSecond: *speed * 50, Acceleration: *acceleration}
	started := time.Now()
	if *mode == "campaign" {
		w := game.New(levels, *seed)
		w.SetCombatBoundary(combat.Boundary)
		r.Stages = run(w, player.New(config), *limit, true, trace)
		r.Completed = w.State == game.Won
	} else {
		end := min(*last, len(levels))
		if *mode == "combats" {
			end = 6
		}
		r.Completed = true
		for index := *first; index <= end; index++ {
			w := game.New(levels, *seed)
			w.SetCombatBoundary(combat.Boundary)
			if *mode == "combats" {
				err = w.StartCombat(index)
			} else {
				err = w.StartAt(index - 1)
			}
			if err != nil {
				fatal(err)
			}
			stages := run(w, player.New(config), *limit, false, trace)
			r.Stages = append(r.Stages, stages...)
			if len(stages) == 0 || stages[len(stages)-1].Outcome != "cleared" {
				r.Completed = false
			}
		}
	}
	r.WallSeconds = time.Since(started).Seconds()
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		fatal(err)
	}
	if *output != "" {
		if err = os.WriteFile(*output, append(data, '\n'), 0644); err != nil {
			fatal(err)
		}
	}
	fmt.Println(string(data))
	if !r.Completed {
		os.Exit(1)
	}
}

func run(w *game.World, a *player.Agent, limit uint64, campaign bool, trace *json.Encoder) []stage {
	stages := []stage{}
	current := stage{Kind: "round", Number: w.LevelIndex + 1, LivesStart: w.Lives}
	if w.State == game.Combat {
		current.Kind, current.Number = "combat", w.Combat.Number
	}
	start, lastDamage := w.TickCount, w.TickCount
	remainingHP := brickHealth(w)
	for {
		input := a.Next(w)
		combatBefore := w.Combat
		w.Tick(input)
		if trace != nil {
			if err := trace.Encode(struct {
				Tick      uint64           `json:"tick"`
				State     game.State       `json:"state"`
				Level     int              `json:"level"`
				Input     game.Input       `json:"input"`
				Paddle    game.Paddle      `json:"paddle"`
				Balls     []game.Ball      `json:"balls"`
				Events    []game.Event     `json:"events"`
				Lives     int              `json:"lives"`
				Remaining int              `json:"remaining"`
				Combat    *game.CombatData `json:"combat,omitempty"`
			}{w.TickCount, w.State, w.LevelIndex + 1, input, w.Paddle, w.Balls, w.Events, w.Lives, w.RemainingBricks(), w.Combat}); err != nil {
				fatal(err)
			}
		}
		cleared := false
		health := brickHealth(w)
		if health < remainingHP {
			lastDamage = w.TickCount
		}
		remainingHP = health
		for _, event := range w.Events {
			if event.Kind == game.CombatHit && event.Value > 0 {
				lastDamage = w.TickCount
			}
			if event.Kind == game.LevelCompleted || event.Kind == game.CombatCompleted {
				cleared = true
			}
		}
		outcome := ""
		if cleared {
			outcome = "cleared"
		} else if w.State == game.GameOver {
			outcome = "game_over"
		} else if w.TickCount-start >= limit {
			outcome = "time_limit"
		} else if w.TickCount-lastDamage >= 30000 {
			outcome = "stagnation"
		}
		if outcome != "" {
			current.Ticks, current.Seconds = w.TickCount-start, float64(w.TickCount-start)/50
			current.LivesEnd, current.Remaining, current.Score, current.Outcome = w.Lives, w.RemainingBricks(), w.Score, outcome
			if current.Kind == "combat" {
				current.Remaining = 0
				if combatBefore != nil {
					current.PlayerEnergy = combatBefore.PlayerEnergy
					current.BossEnergy = combatBefore.BossEnergy
				}
			}
			if outcome != "cleared" {
				current.Failure = &failure{Paddle: w.Paddle, Balls: append([]game.Ball(nil), w.Balls...), Effects: append([]game.Effect(nil), w.Effects...), Enemies: append([]game.Enemy(nil), w.Enemies...), Combat: w.Combat}
				for _, b := range w.Bricks {
					if !b.Destroyed {
						current.Failure.Bricks = append(current.Failure.Bricks, b)
					}
				}
			}
			stages = append(stages, current)
			fmt.Fprintf(os.Stderr, "%s %02d: %s, %.1f s, lives %d -> %d, %d bricks\n", current.Kind, current.Number, outcome, current.Seconds, current.LivesStart, current.LivesEnd, current.Remaining)
			if !campaign || outcome != "cleared" || w.State == game.Won {
				return stages
			}
			for w.State == game.LevelClear {
				w.Tick(a.Next(w))
			}
			current = stage{Kind: "round", Number: w.LevelIndex + 1, LivesStart: w.Lives}
			if w.State == game.Combat {
				current.Kind, current.Number = "combat", w.Combat.Number
			}
			start, lastDamage = w.TickCount, w.TickCount
			remainingHP = brickHealth(w)
		}
	}
}

func brickHealth(w *game.World) int {
	health := 0
	for _, brick := range w.Bricks {
		if brick.Destructible && !brick.Destroyed {
			health += max(1, brick.HP)
		}
	}
	return health
}

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(2) }
