package presentation

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"krytonegg/internal/game"
	"krytonegg/internal/touch"
)

// Construction edits the original 18x16 grid using the disk's tile identifiers.
// It writes the original big-endian format, so no new artwork is introduced.
type Construction struct {
	cells    [game.LevelColumns * game.LevelRows]uint16
	selected int
	palette  []int
	bonus    int
}

func (a *App) openEditor() {
	c := &Construction{}
	for _, start := range []int{1, 17, 33, 49, 65, 97, 241} {
		count := 13
		if start == 241 {
			count = 13
		}
		for i := 0; i < count; i++ {
			c.palette = append(c.palette, start+i)
		}
	}
	offset := a.selectedRound * game.LevelByteSize
	for i := range c.cells {
		c.cells[i] = binary.BigEndian.Uint16(a.table[offset+i*2 : offset+i*2+2])
	}
	a.construction, a.editor, a.help = c, true, false
	a.audio.Music("")
}

func (c *Construction) encode() []byte {
	data := make([]byte, game.LevelByteSize)
	for i, value := range c.cells {
		binary.BigEndian.PutUint16(data[i*2:i*2+2], value)
	}
	return data
}

func (a *App) updateEditor() error {
	c := a.construction
	save, load, test := false, false, false
	if a.options.Mobile {
		for _, action := range a.touchFrame.Actions {
			switch action {
			case touch.TilePrev:
				c.selected = (c.selected + len(c.palette) - 1) % len(c.palette)
			case touch.TileNext:
				c.selected = (c.selected + 1) % len(c.palette)
			case touch.Bonus:
				c.bonus = (c.bonus + 4) % 112
			case touch.Power:
				c.bonus = (c.bonus &^ 3) | ((c.bonus + 1) & 3)
			case touch.Erase:
				a.touchErase = !a.touchErase
			case touch.Save:
				save = true
			case touch.Load:
				load = true
			case touch.Test:
				test = true
			}
		}
	}
	if a.key(ebiten.KeyArrowLeft) {
		c.selected = (c.selected + len(c.palette) - 1) % len(c.palette)
	}
	if a.key(ebiten.KeyArrowRight) {
		c.selected = (c.selected + 1) % len(c.palette)
	}
	_, wheel := ebiten.Wheel()
	if wheel > 0 {
		c.selected = (c.selected + 1) % len(c.palette)
	}
	if wheel < 0 {
		c.selected = (c.selected + len(c.palette) - 1) % len(c.palette)
	}
	if a.key(ebiten.KeyB) {
		c.bonus = (c.bonus + 4) % 112
	}
	if a.key(ebiten.KeyV) {
		c.bonus = (c.bonus &^ 3) | ((c.bonus + 1) & 3)
	}
	mx, my := ebiten.CursorPosition()
	x, y := a.graphics.WorldPosition(mx, my)
	if a.options.Mobile {
		x, y = a.touchFrame.PaintX, a.touchFrame.PaintY
	}
	column, row := int((x-game.FieldLeft)/16), int((y-24)/8)
	if x >= game.FieldLeft && x < game.FieldRight && y >= 24 && y < 152 && row >= 0 && row < game.LevelRows && column >= 0 && column < game.LevelColumns {
		i := row*game.LevelColumns + column
		if (!a.options.Mobile && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)) || (a.options.Mobile && a.touchFrame.Painting && !a.touchErase) {
			c.cells[i] = uint16(c.palette[c.selected]) | uint16(c.bonus)<<8
		}
		if (!a.options.Mobile && ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)) || (a.options.Mobile && a.touchFrame.Painting && a.touchErase) {
			c.cells[i] = 0
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyS) || save {
		if err := atomicWrite(filepath.Join(a.options.DataDir, "custom.level"), c.encode()); err != nil {
			a.message, a.messageTicks = "SAVE FAILED", 180
		} else {
			a.message, a.messageTicks = "LEVEL SAVED", 180
		}
	}
	if a.key(ebiten.KeyL) || load {
		data, err := os.ReadFile(filepath.Join(a.options.DataDir, "custom.level"))
		if err != nil || len(data) != game.LevelByteSize {
			a.message, a.messageTicks = "NO SAVED LEVEL", 180
		} else {
			for i := range c.cells {
				c.cells[i] = binary.BigEndian.Uint16(data[i*2 : i*2+2])
			}
		}
	}
	if a.key(ebiten.KeyEnter) || test {
		levels, err := game.LoadLevels(c.encode())
		if err != nil {
			return err
		}
		if count := countDestructible(levels[0]); count == 0 {
			a.message, a.messageTicks = "ADD BREAKABLE BRICKS", 180
			return nil
		}
		a.world = game.New(levels, a.options.Seed)
		a.world.Restart()
		a.editor, a.testing = false, true
	}
	return nil
}

func countDestructible(level game.Level) int {
	count := 0
	for _, brick := range level.Bricks {
		if brick.Destructible {
			count++
		}
	}
	return count
}

func (a *App) drawEditor(screen *ebiten.Image) {
	g, c := a.graphics, a.construction
	g.Image(screen, fmt.Sprintf("images/background-%d.png", a.selectedRound%83), 0, 0)
	for i, code := range c.cells {
		if code&255 == 0 {
			continue
		}
		g.Image(screen, fmt.Sprintf("sprites/brick-%d.png", code&255), game.FieldLeft+float64(i%game.LevelColumns)*16, 24+float64(i/game.LevelColumns)*8)
	}
	// Clear text fields so editor instructions cannot overlap the original HUD
	// digits or the patterned field beneath the editable brick grid.
	g.Clear(screen, 0, 0, 320, 16)
	g.Clear(screen, 16, 154, 288, 46)
	g.CenteredText(screen, "CONSTRUCTION", 4)
	g.Image(screen, fmt.Sprintf("sprites/brick-%d.png", c.palette[c.selected]), 16, 158)
	if a.options.Mobile {
		g.CenteredText(screen, a.editorTouchInfo(), 158)
		g.CenteredText(screen, "DRAG TO PAINT BRICKS", 174)
		g.CenteredText(screen, "USE SIDE BUTTONS TO EDIT", 188)
		return
	}
	g.Text(screen, fmt.Sprintf("TILE %03d BONUS %02d POWER %d", c.palette[c.selected], c.bonus>>2, c.bonus&3), 40, 158)
	g.CenteredText(screen, "ARROWS TILE B BONUS V POWER", 170)
	g.CenteredText(screen, "S SAVE  L LOAD  ENTER TEST", 182)
	if a.messageTicks > 0 {
		return
	}
	g.CenteredText(screen, "RIGHT CLICK ERASE  ESC BACK", 192)
}

// atomicWrite protects a saved score or level against interrupted writes.
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".krytonegg-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
