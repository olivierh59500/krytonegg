package presentation

import (
	"testing"

	"krytonegg/assets"
	"krytonegg/internal/game"
	"krytonegg/internal/player"
	"krytonegg/internal/scores"
)

func workflowApp(t *testing.T) *App {
	t.Helper()
	a := mobileTestApp(t)
	menu, err := assets.Read("original/menu.art")
	if err != nil {
		t.Fatal(err)
	}
	table, err := scores.DecodeOriginal(menu)
	if err != nil {
		t.Fatal(err)
	}
	a.presentation = &Presentation{Table: table, Intro: IntroSequence{Phase: IntroFinished}, directory: a.options.DataDir}
	return a
}

func TestEndOfRunRequestsNameBeforeInsertingScore(t *testing.T) {
	a := workflowApp(t)
	original := a.presentation.Table
	a.world.State, a.world.Score = game.GameOver, 10000
	a.offerRunScore()
	if !a.nameActive() || a.presentation.Table != original {
		t.Fatal("qualifying score was inserted without name confirmation")
	}
	a.presentation.Name.Type("TEST PLAYER")
	if err := a.presentation.FinishName(); err != nil {
		t.Fatal(err)
	}
	a.offerRunScore()
	if a.nameActive() || a.presentation.Table.Entries[0] != (scores.Entry{Name: "TEST PLAYER", Score: 10000}) {
		t.Fatal("the same completed run requested another name")
	}
}

func TestBackgroundCursorSurvivesReturningToTitle(t *testing.T) {
	a := workflowApp(t)
	a.textureCursor = 82
	a.world.Restart()
	a.consumeEvents()
	if a.displayIndex != 82 || a.textureCursor != 0 || !a.levelBanner {
		t.Fatal("first display did not consume the next original texture")
	}
	a.returnToTitle()
	a.world.Restart()
	a.consumeEvents()
	if a.displayIndex != 0 || a.textureCursor != 1 {
		t.Fatal("returning to the title reset the original texture cursor")
	}
}

func TestLevelBannerFreezesSimulationUntilAcknowledged(t *testing.T) {
	a := workflowApp(t)
	a.options.Expert = true
	a.pilot = player.New(player.Config{Seed: 1990})
	a.world.Restart()
	a.consumeEvents()
	for i := 0; i < 50; i++ {
		if err := a.Update(); err != nil {
			t.Fatal(err)
		}
		if a.world.TickCount != 0 {
			t.Fatal("actors advanced behind the pre-paddle level banner")
		}
	}
	if a.levelBanner {
		t.Fatal("expert UI did not acknowledge the banner")
	}
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	if a.world.TickCount != 1 {
		t.Fatal("simulation did not resume after the level banner")
	}
}
