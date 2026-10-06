// Package presentation draws original Amiga assets at the window's resolution.
package presentation

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"krytonegg/assets"
	"krytonegg/internal/game"
)

// Graphics keeps GPU copies of the original bitmaps and their visible bounds.
type Graphics struct {
	images                  map[string]*ebiten.Image
	bounds                  map[string]image.Rectangle
	font                    [36]*ebiten.Image
	digits                  [10]*ebiten.Image
	bonusFrames             map[int]int
	stars                   []star
	animationTick           uint64
	Scale, OffsetX, OffsetY float64
}

type star struct {
	x, y float64
	dot  bool
}

// SetStars places the original 61 stars and 76 blue dots once per run. The
// original obtains positions from raster timing; the port uses a replay seed.
func (g *Graphics) SetStars(seed uint64) {
	if seed == 0 {
		seed = 1
	}
	random := func(limit uint64) float64 {
		seed ^= seed << 13
		seed ^= seed >> 7
		seed ^= seed << 17
		return float64(seed % limit)
	}
	g.stars = g.stars[:0]
	for i := 0; i < 137; i++ {
		g.stars = append(g.stars, star{x: random(222), y: 8 + random(143), dot: i >= 61})
	}
}

// NewGraphics converts embedded PNG data to textures without repainting it.
func NewGraphics() (*Graphics, error) {
	g := &Graphics{images: make(map[string]*ebiten.Image), bounds: make(map[string]image.Rectangle), bonusFrames: make(map[int]int)}
	names, err := assets.Names()
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if !strings.HasSuffix(name, ".png") {
			continue
		}
		img, err := assets.Image(name)
		if err != nil {
			return nil, err
		}
		g.images[name] = ebiten.NewImageFromImage(img)
		// Masked sprites reserve blitter padding. Its transparent columns must
		// not count toward the visible paddle size or ball's center position.
		b := img.Bounds()
		if strings.HasPrefix(name, "sprites/sprite-") {
			var visible image.Rectangle
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					_, _, _, a := img.At(x, y).RGBA()
					if a != 0 {
						visible = visible.Union(image.Rect(x, y, x+1, y+1))
					}
				}
			}
			b = visible
		}
		g.bounds[name] = b
	}
	font := g.images["sprites/font.png"]
	digits := g.images["sprites/digits.png"]
	if font == nil || digits == nil || g.images["images/background-0.png"] == nil {
		return nil, fmt.Errorf("missing original bitmap font or background; run make assets")
	}
	for i := range g.font {
		g.font[i] = font.SubImage(image.Rect(i*8, 0, (i+1)*8, 8)).(*ebiten.Image)
	}
	for i := range g.digits {
		g.digits[i] = digits.SubImage(image.Rect(i*8, 0, (i+1)*8, 8)).(*ebiten.Image)
	}
	for kind := 1; kind <= 27; kind++ {
		for frame := 0; g.images[fmt.Sprintf("sprites/bonus-%d-%d.png", kind, frame)] != nil; frame++ {
			g.bonusFrames[kind] = frame + 1
		}
	}
	// Read the original mask extents instead of deriving positions from padded
	// blitter widths. Source offsets remain documented alongside each bitmap.
	data, err := assets.Read("manifest.json")
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Sprites map[string]struct {
			Bounds [4]int `json:"opaque_bounds"`
		} `json:"sprites"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	for name, item := range manifest.Sprites {
		g.bounds["sprites/"+name+".png"] = image.Rect(item.Bounds[0], item.Bounds[1], item.Bounds[2], item.Bounds[3])
	}
	return g, nil
}

// Layout computes integer zoom and centered letterboxing for crisp pixel art.
func (g *Graphics) Layout(width, height int) {
	g.Scale = math.Min(float64(width)/game.Width, float64(height)/game.Height)
	if g.Scale >= 1 {
		g.Scale = math.Floor(g.Scale)
	}
	g.OffsetX = (float64(width) - game.Width*g.Scale) / 2
	g.OffsetY = (float64(height) - game.Height*g.Scale) / 2
}

// WorldPosition converts mouse and touch coordinates to the original playfield.
func (g *Graphics) WorldPosition(x, y int) (float64, float64) {
	if g.Scale == 0 {
		return 160, 190
	}
	return (float64(x) - g.OffsetX) / g.Scale, (float64(y) - g.OffsetY) / g.Scale
}

func (g *Graphics) draw(screen *ebiten.Image, img *ebiten.Image, x, y, sx, sy float64) {
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(g.Scale*sx, g.Scale*sy)
	op.GeoM.Translate(g.OffsetX+x*g.Scale, g.OffsetY+y*g.Scale)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(img, op)
}

// Image draws an original bitmap at native world coordinates.
func (g *Graphics) Image(screen *ebiten.Image, name string, x, y float64) {
	g.draw(screen, g.images[name], x, y, 1, 1)
}

// Sprite aligns a recovered masked sprite by its actual visible top-left pixel.
func (g *Graphics) Sprite(screen *ebiten.Image, index int, x, y float64) {
	name := fmt.Sprintf("sprites/sprite-%02d.png", index)
	b := g.bounds[name]
	g.Image(screen, name, x-float64(b.Min.X), y-float64(b.Min.Y))
}

// Text uses the original game alphabet, including its original colors.
func (g *Graphics) Text(screen *ebiten.Image, str string, x, y float64) {
	for _, ch := range strings.ToUpper(str) {
		index := -1
		if ch >= 'A' && ch <= 'Z' {
			index = int(ch - 'A')
		}
		if ch >= '0' && ch <= '9' {
			index = int(ch-'0') + 26
		}
		if index >= 0 {
			g.draw(screen, g.font[index], x, y, 1, 1)
		}
		x += 8
	}
}

// CenteredText positions an original bitmap-font message within the game frame.
func (g *Graphics) CenteredText(screen *ebiten.Image, str string, y float64) {
	g.Text(screen, str, float64(game.Width-len(str)*8)/2, y)
}

func (g *Graphics) number(screen *ebiten.Image, value, width int, x, y float64) {
	g.Clear(screen, x, y, float64(width*8), 8)
	if value < 0 {
		value = 0
	}
	str := fmt.Sprintf("%0*d", width, value)
	if len(str) > width {
		str = str[len(str)-width:]
	}
	for _, ch := range str {
		g.draw(screen, g.digits[ch-'0'], x, y, 1, 1)
		x += 8
	}
}

// Clear restores a black text field before writing transparent original glyphs.
func (g *Graphics) Clear(screen *ebiten.Image, x, y, width, height float64) {
	rect := image.Rect(int(g.OffsetX+x*g.Scale), int(g.OffsetY+y*g.Scale), int(g.OffsetX+(x+width)*g.Scale), int(g.OffsetY+(y+height)*g.Scale))
	rect = rect.Intersect(screen.Bounds())
	if !rect.Empty() {
		screen.SubImage(rect).(*ebiten.Image).Fill(color.Black)
	}
}

func (g *Graphics) hud(screen *ebiten.Image, w *game.World, highScore int) {
	g.number(screen, w.Score, 6, 55, 4)
	g.number(screen, max(0, w.Lives-1), 2, 151, 4)
	g.number(screen, max(highScore, w.Score), 6, 263, 4)
}

func (g *Graphics) paddle(screen *ebiten.Image, paddle game.Paddle, w *game.World) {
	bank := "normal"
	if w.Active(game.Sticky) {
		bank = "magnet"
	}
	if w.Active(game.Laser) || w.Active(game.Rocket) || w.Active(game.EnhancedCannon) {
		bank = "weapon"
	}
	slot := paddle.Slot
	if slot < 6 || slot > 24 {
		slot = 13
	}
	g.Image(screen, fmt.Sprintf("sprites/paddle-%s-%d.png", bank, slot), paddle.X-paddle.W/2, paddle.Y-paddle.H/2)
}

// Board draws only disk graphics. Floating-point world positions retain smooth
// motion at higher output resolutions while source texels remain sharp.
func (g *Graphics) Board(screen *ebiten.Image, w *game.World, highScore int) {
	if w.State != game.Paused {
		g.animationTick = w.TickCount
	}
	if !w.Active(game.Darkness) {
		g.Image(screen, fmt.Sprintf("images/background-%d.png", w.LevelIndex%83), 0, 0)
		for _, b := range w.Bricks {
			if !b.Destroyed {
				g.Image(screen, fmt.Sprintf("sprites/brick-%d.png", b.Kind), b.X, b.Y)
			}
		}
	} else {
		g.Image(screen, "images/hud.png", 0, 0)
		g.Image(screen, "images/wall-left.png", 0, 24)
		g.Image(screen, "images/wall-right.png", 304, 24)
	}
	if w.DoorFrame > 0 {
		g.Image(screen, fmt.Sprintf("sprites/door-%d.png", w.DoorFrame), 144, 16)
	}
	for _, enemy := range w.Enemies {
		if enemy.Destroyed {
			continue
		}
		name := fmt.Sprintf("sprites/enemy-%d-%d.png", enemy.Kind, enemy.Frame)
		if g.images[name] == nil {
			name = fmt.Sprintf("sprites/enemy-%d-0.png", enemy.Kind)
		}
		g.Image(screen, name, enemy.X, enemy.Y)
	}
	for _, drop := range w.Drops {
		// The original high-byte encodes the falling bonus graphic and strength.
		kind := int(drop.Code) >> 2
		name := fmt.Sprintf("sprites/bonus-%d.png", kind)
		if count := g.bonusFrames[kind]; count > 1 {
			name = fmt.Sprintf("sprites/bonus-%d-%d.png", kind, int(g.animationTick/4)%count)
		}
		g.Image(screen, name, drop.X-8, drop.Y-4)
	}
	for _, shot := range w.Shots {
		name := "sprites/laser.png"
		if shot.Width > 5 {
			name = "sprites/laser-alt.png"
		}
		b := g.bounds[name]
		g.Image(screen, name, shot.X-float64(b.Dx())/2, shot.Y-float64(b.Dy()))
	}
	for _, ball := range w.Balls {
		index := 0
		for i, diameter := range []float64{5, 6, 8, 10, 13, 16} {
			if ball.Radius*2 >= diameter {
				index = i
			}
		}
		name := fmt.Sprintf("sprites/sprite-%02d.png", index)
		if w.Active(game.SuperBall) {
			name = fmt.Sprintf("sprites/superball-%d.png", index)
		}
		if w.Active(game.GhostBall) {
			name = fmt.Sprintf("sprites/ghostball-%d.png", index)
		}
		// The original bitmap adds a two-pixel shadow on its right and bottom.
		g.Image(screen, name, ball.X-ball.Radius, ball.Y-ball.Radius)
	}
	if w.Active(game.GhostPaddle) {
		g.Image(screen, "sprites/paddle-ghost.png", w.Paddle.X-16, w.Paddle.Y-4)
	} else {
		g.paddle(screen, w.Paddle, w)
	}
	if w.SecondPaddle != nil {
		g.paddle(screen, *w.SecondPaddle, w)
	}
	g.hud(screen, w, highScore)
}

// Combat draws the original organic boss scene and original shot sprites.
func (g *Graphics) Combat(screen *ebiten.Image, w *game.World, highScore int) {
	c := w.Combat
	if c == nil {
		return
	}
	g.Image(screen, "images/combat.png", 0, 0)
	starImage := g.images["sprites/combat-star.png"]
	if starImage != nil {
		dot := starImage.SubImage(image.Rect(1, 0, 2, 1)).(*ebiten.Image)
		for _, star := range g.stars {
			// The original alien rectangles cover stars beneath the top and
			// bottom organic walls after the starfield has been initialized.
			if star.x >= 160 && (star.y < 17 || star.y >= 144) {
				continue
			}
			img := starImage
			if star.dot {
				img = dot
			}
			g.draw(screen, img, star.x, star.y, 1, 1)
		}
	}
	g.Image(screen, fmt.Sprintf("sprites/combat-mouth-%d.png", c.BossFrame), 240, 55)
	g.Image(screen, "sprites/combat-ship.png", c.ShipX, c.ShipY)
	for _, shot := range c.PlayerShots {
		g.Image(screen, "sprites/combat-shot.png", shot.X, shot.Y)
	}
	for _, shot := range c.BossShots {
		g.Image(screen, "sprites/combat-enemy-shot.png", shot.X-8, shot.Y-6)
	}
	meters := g.images["images/combat-meters.png"]
	if meters != nil {
		// Crop the original meter fill to the surviving energy; retain the
		// original frame, blue player fill, and pink alien fill unchanged.
		for _, bar := range []struct{ x, energy, maximum int }{{27, c.PlayerEnergy, 448}, {180, c.BossEnergy, 896}} {
			g.Clear(screen, float64(bar.x), 179, 112, 7)
			width := max(0, min(112, bar.energy*112/bar.maximum))
			if width > 0 {
				g.draw(screen, meters.SubImage(image.Rect(bar.x, 12, bar.x+width, 19)).(*ebiten.Image), float64(bar.x), 179, 1, 1)
			}
		}
	}
}
