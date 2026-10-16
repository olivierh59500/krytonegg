package presentation

import (
	"github.com/hajimehoshi/ebiten/v2"

	"krytonegg/internal/game"
)

type originalCodeInput struct {
	authorize, unlimited, skip, final, weaken bool
}

// enforceAutomaticMode prevents optional original codes from affecting expert
// validation, smoke checks, or any recording, including a movie-only invocation.
func (a *App) enforceAutomaticMode() {
	if a.automatic() {
		a.CheatAuthorized = false
		a.world.UnlimitedReserves = false
	}
}

func (a *App) updateOriginalCodes() bool {
	if a.automatic() {
		return false
	}
	return a.applyOriginalCodes(originalCodeInput{
		authorize: a.introActive() && ebiten.IsKeyPressed(ebiten.KeyInsert) && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft),
		unlimited: a.key(ebiten.KeyF10), skip: a.key(ebiten.KeyEscape),
		final: a.key(ebiten.KeyControlLeft) || a.key(ebiten.KeyControlRight), weaken: a.key(ebiten.KeyF8),
	})
}

// applyOriginalCodes handles explicitly authorized manual controls. Separating
// the intent from hardware polling makes the automatic-mode boundary testable.
func (a *App) applyOriginalCodes(input originalCodeInput) bool {
	a.enforceAutomaticMode()
	if a.automatic() {
		return false
	}
	if input.authorize && a.introActive() && !a.CheatAuthorized {
		a.CheatAuthorized = true
		a.message, a.messageTicks = "ORIGINAL CODES ENABLED", 180
	}
	if !a.CheatAuthorized || a.introActive() || a.editor || a.testing || a.help || a.scores || a.credits || a.nameActive() || a.interludeActive() {
		return false
	}
	normal := a.world.State == game.Ready || a.world.State == game.Playing || a.world.State == game.LevelClear || a.world.State == game.Dying
	if input.unlimited && normal {
		a.world.EnableUnlimitedReserves()
		a.message, a.messageTicks = "UNLIMITED RESERVES", 180
		return true
	}
	if input.final && normal {
		a.world.Events = a.world.Events[:0]
		a.world.CheatFinalCombat()
		a.levelBanner = false
		a.consumeEvents()
		return true
	}
	if input.skip && normal {
		a.world.Events = a.world.Events[:0]
		a.world.CheatSkipRound()
		a.levelBanner = false
		a.consumeEvents()
		return true
	}
	if input.weaken && a.world.State == game.Combat {
		a.world.CheatWeakenBoss()
		a.message, a.messageTicks = "ORIGINAL BOSS CODE", 180
		return true
	}
	return false
}
