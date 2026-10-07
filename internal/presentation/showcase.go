package presentation

import (
	"fmt"
	"time"

	"krytonegg/internal/game"
	"krytonegg/internal/player"
)

// Showcase sequences reviewable UI actions around genuine expert-player inputs.
// Gameplay uses the same bounded controller as the progression validation.
type Showcase struct {
	phase    int
	prepared bool
}

func (a *App) newPilot() *player.Agent {
	speed, reaction := a.options.PlayerSpeed, a.options.PlayerReaction
	if speed <= 0 {
		speed = 10
	}
	if reaction <= 0 {
		reaction = 5
	}
	return player.New(player.Config{Seed: a.options.Seed, MaxSpeed: speed, ReactionTicks: reaction})
}

// Update presents thirty seconds of options, 190 seconds of uninterrupted
// campaign inputs, and a clearly identified twenty-second practice encounter.
// It never moves a ball, destroys a brick, awards lives, or changes difficulty.
func (s *Showcase) Update(a *App) bool {
	if !s.prepared {
		s.prepared = true
		speed, reaction := a.options.PlayerSpeed, a.options.PlayerReaction
		if speed <= 0 {
			speed = 10
		}
		if reaction <= 0 {
			reaction = 5
		}
		if a.recorder != nil {
			for _, cue := range []struct {
				start, end int
				text       string
			}{
				{0, 5, "KRYPTON EGG\nA native Go and Ebitengine remake of the Amiga game"},
				{5, 8, "Original artwork and credits, reconstructed from your local ADF"},
				{8, 13, "The original menu and its animated Amiga color effects"},
				{13, 17, "The ten-entry Hall of Fame preserves the original names and scores"},
				{17, 21, "Mouse or keyboard, pause, PAL and NTSC, fullscreen and sound controls"},
				{21, 26, "The construction set edits original bricks, bonuses and strength"},
				{26, 30, "Android: drag to steer, hold FIRE with a second finger"},
				{30, 38, "CAMPAIGN PLAY\nA strong simulated player uses only ordinary game inputs"},
				{38, 48, fmt.Sprintf("%d ms reaction delay, %.0f original pixels/s and bounded acceleration\nNo injected lives or bonuses, no brick removal, no skipped rounds", reaction*20, speed*50)},
				{58, 68, "Original layered bricks, falling bonuses and enemy encounters"},
				{83, 93, "Native integer ball velocities and paddle-motion rebounds"},
				{108, 118, "The player predicts landings and chooses bonuses while avoiding hazards"},
				{138, 148, "Original graphics and sound samples; tracker music is replayed by go-zikmu"},
				{173, 183, "The same game simulation powers desktop and Android versions"},
				{205, 219, "This campaign segment runs continuously at the original PAL update rate"},
				{220, 227, "PRACTICE ENCOUNTER\nThe original first boss, selected separately for this presentation"},
				{228, 239, "Original boss energy, projectiles and timing\nThe same bounded player steers and fires without invulnerability"},
			} {
				_ = a.recorder.AddCue(time.Duration(cue.start)*time.Second, time.Duration(cue.end)*time.Second, cue.text)
			}
		}
	}
	tick := a.steps
	phase := 0
	for _, boundary := range []int{400, 650, 850, 1050, 1300, 1500, 11000} {
		if tick >= boundary {
			phase++
		}
	}
	if phase != s.phase {
		s.phase = phase
		switch phase {
		case 1:
			a.presentation.SkipIntro()
		case 2:
			a.scores = true
		case 3:
			a.scores = false
			a.help = true
		case 4:
			a.help = false
			a.openEditor()
		case 5:
			a.Back()
			a.options.Mobile = true
		case 6:
			a.options.Mobile = false
			a.scores = false
			a.help = false
			a.credits = false
			a.presentation.Name.Active = false
			a.pilot = a.newPilot()
			_ = a.world.StartAt(0)
			a.runScoreRecorded = false
			a.consumeEvents()
		case 7:
			// Practice selection is explicit in the subtitle; it is excluded
			// from the uninterrupted campaign progression report.
			a.presentation.Interlude.Active = false
			a.presentation.Name.Active = false
			a.levelBanner = false
			a.scores = false
			a.help = false
			a.pilot = a.newPilot()
			_ = a.world.StartCombat(1)
			a.runScoreRecorded = false
			a.consumeEvents()
			a.levelBanner = false
		}
	}
	if phase == 0 {
		if tick == 250 {
			a.presentation.Intro.Update(true)
		}
		return false
	}
	if phase < 6 {
		return true
	}
	// A qualifying score may occur naturally at the end of the recorded run.
	if a.nameActive() {
		a.presentation.Name.Type("EXPERT PLAYER")
		_ = a.presentation.FinishName()
		a.scores = true
		return true
	}
	if a.world.State == game.GameOver || a.world.State == game.Won {
		return true
	}
	return false
}
