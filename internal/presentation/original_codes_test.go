package presentation

import (
	"testing"

	"krytonegg/internal/game"
)

func TestOriginalCodesRequireTheIntroAuthorization(t *testing.T) {
	a := workflowApp(t)
	a.world.Restart()
	if a.applyOriginalCodes(originalCodeInput{authorize: true, unlimited: true, skip: true}) || a.CheatAuthorized || a.world.UnlimitedReserves || a.world.LevelIndex != 0 {
		t.Fatal("codes activated outside the startup intro")
	}
	a.presentation.Intro.Phase = IntroReveal
	a.applyOriginalCodes(originalCodeInput{authorize: true})
	if !a.CheatAuthorized {
		t.Fatal("the explicit startup authorization was ignored")
	}
	a.presentation.SkipIntro()
	if !a.applyOriginalCodes(originalCodeInput{unlimited: true}) || !a.world.UnlimitedReserves {
		t.Fatal("authorized reserve code did not activate")
	}
	a.returnToTitle()
	if !a.CheatAuthorized || !a.world.UnlimitedReserves {
		t.Fatal("original process-persistent authorization or reserves were reset")
	}
}

func TestAutomaticModesRemoveAndRejectOriginalCodes(t *testing.T) {
	for name, options := range map[string]Options{
		"expert": {Expert: true}, "showcase": {Showcase: true}, "smoke": {SmokeTicks: 1}, "movie only": {Movie: "unused.mp4"},
	} {
		t.Run(name, func(t *testing.T) {
			a := workflowApp(t)
			a.options.Expert, a.options.Showcase, a.options.SmokeTicks, a.options.Movie = options.Expert, options.Showcase, options.SmokeTicks, options.Movie
			a.CheatAuthorized = true
			a.world.EnableUnlimitedReserves()
			a.resetWorld()
			if a.CheatAuthorized || a.world.UnlimitedReserves {
				t.Fatal("a recording or validation inherited optional cheat state")
			}
			a.presentation.Intro.Phase = IntroReveal
			if a.applyOriginalCodes(originalCodeInput{authorize: true, unlimited: true, skip: true, final: true, weaken: true}) || a.CheatAuthorized || a.world.UnlimitedReserves {
				t.Fatal("automatic input isolation allowed original codes")
			}
			if a.gamepadConfirm() {
				t.Fatal("an automatic modal accepted controller confirmation")
			}
		})
	}
}

func TestManualSkipKeepsTenthRoundBossAndRequiresRealShots(t *testing.T) {
	a := workflowApp(t)
	a.CheatAuthorized = true
	if err := a.world.StartAt(9); err != nil {
		t.Fatal(err)
	}
	a.world.Score = 123
	lives := a.world.Lives
	if !a.applyOriginalCodes(originalCodeInput{skip: true}) || a.world.State != game.Combat || a.world.Combat.Number != 1 {
		t.Fatal("the original round skip bypassed its tenth-round boss")
	}
	if a.world.Score != 123 || a.world.Lives != lives {
		t.Fatal("manual stage skip awarded points or reserves")
	}
	if !a.applyOriginalCodes(originalCodeInput{weaken: true}) || a.world.Combat.BossEnergy != 8 || a.world.State != game.Combat {
		t.Fatal("boss code completed the encounter instead of requiring shots")
	}
	if err := a.world.StartAt(0); err != nil {
		t.Fatal(err)
	}
	if !a.applyOriginalCodes(originalCodeInput{final: true}) || a.world.State != game.Combat || a.world.Combat.Number != 6 {
		t.Fatal("authorized final-encounter selection failed")
	}
}
