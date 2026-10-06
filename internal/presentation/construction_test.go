package presentation

import (
	"bytes"
	"encoding/binary"
	"testing"

	"krytonegg/assets"
	"krytonegg/internal/game"
)

func TestConstructionPreservesOriginalFormat(t *testing.T) {
	table, err := assets.Read("original/level.tab")
	if err != nil {
		t.Fatal(err)
	}
	c := &Construction{}
	for i := range c.cells {
		c.cells[i] = binary.BigEndian.Uint16(table[i*2 : i*2+2])
	}
	if !bytes.Equal(c.encode(), table[:game.LevelByteSize]) {
		t.Fatal("construction changed original bonus or tile bits")
	}
	c.cells[12] = 0x5a31
	levels, err := game.LoadLevels(c.encode())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, brick := range levels[0].Bricks {
		if brick.Code == 0x5a31 && brick.X == 208 && brick.Y == 24 {
			found = true
		}
	}
	if !found {
		t.Fatal("custom brick or its encoded bonus was lost")
	}
}

func TestEditorTestExitRestoresFullCampaign(t *testing.T) {
	table, err := assets.Read("original/level.tab")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := assets.Read("levels.json")
	if err != nil {
		t.Fatal(err)
	}
	campaign, err := game.LoadCampaign(table, metadata)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{campaign: campaign, world: game.New(campaign[:1], 1), selectedRound: 59}
	a.resetWorld()
	if len(a.world.Levels) != 60 || a.selectedRound != 59 {
		t.Fatal("editor test replaced or truncated original campaign")
	}
	if err := a.world.StartAt(59); err != nil {
		t.Fatal(err)
	}
	if len(a.world.Levels[59].EnemyChoices) != 4 {
		t.Fatal("restored campaign lost original enemy metadata")
	}
}

func TestHighResolutionLetterboxInputMapping(t *testing.T) {
	g := &Graphics{}
	g.Layout(1500, 900)
	if g.Scale != 4 || g.OffsetX != 110 || g.OffsetY != 50 {
		t.Fatalf("incorrect integer zoom: %+v", g)
	}
	x, y := g.WorldPosition(750, 810)
	if x != 160 || y != 190 {
		t.Fatalf("mouse does not align with source coordinates: %g,%g", x, y)
	}
}
