package presentation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"krytonegg/assets"
	"krytonegg/internal/game"
	"krytonegg/internal/scores"
)

// IntroPhase follows the original image reveal, click wait, and credits scroll.
type IntroPhase uint8

const (
	IntroReveal IntroPhase = iota
	IntroWait
	IntroCredits
	IntroFinished
)

// IntroSequence advances only presentation time and never advances the world.
type IntroSequence struct {
	Phase IntroPhase
	Ticks int
	Lines []string
}

// Update accepts a fire edge; either mouse button skipped the original scroll.
func (s *IntroSequence) Update(fire bool) {
	if s.Phase == IntroFinished {
		return
	}
	if fire {
		if s.Phase == IntroReveal || s.Phase == IntroWait {
			s.Phase, s.Ticks = IntroCredits, 0
		} else {
			s.Phase, s.Ticks = IntroFinished, 0
		}
		return
	}
	s.Ticks++
	if s.Phase == IntroReveal && s.Ticks >= 200 {
		s.Phase, s.Ticks = IntroWait, 0
	}
	if s.Phase == IntroCredits && s.Ticks >= len(s.Lines)*8+200 {
		s.Phase, s.Ticks = IntroFinished, 0
	}
}

// NameEntry supports native keyboard and touch selection of original glyphs.
type NameEntry struct {
	Active      bool
	Score, Rank int
	Name        string
}

// Type accepts only the disk font's alphabet, digits, and spaces.
func (e *NameEntry) Type(text string) {
	for _, ch := range strings.ToUpper(text) {
		if len(e.Name) >= scores.NameLength {
			break
		}
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == ' ' {
			e.Name += string(ch)
		}
	}
}

// Backspace removes one original-width character.
func (e *NameEntry) Backspace() {
	if len(e.Name) > 0 {
		e.Name = e.Name[:len(e.Name)-1]
	}
}

const nameKeys = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Touch handles three twelve-column rows plus erase and confirm controls.
// A true result means the confirmation button was selected.
func (e *NameEntry) Touch(x, y float64) bool {
	if x >= 64 && x < 256 && y >= 150 && y < 183 {
		column, row := int(x-64)/16, int(y-150)/11
		e.Type(nameKeys[row*12+column : row*12+column+1])
	} else if y >= 185 && y < 199 {
		if x >= 64 && x < 128 {
			e.Backspace()
		}
		if x >= 136 && x < 192 {
			e.Type(" ")
		}
		return x >= 208 && x < 256
	}
	return false
}

// Interlude retains the six victory clues and the original combat-loss text.
type Interlude struct {
	Active        bool
	Combat, Ticks int
	Lines         []string
}

type presentationData struct {
	MenuCopper                               [][2]uint16 `json:"menu_copper"`
	MenuCycle                                []uint16    `json:"menu_cycle"`
	HighlightOffsets                         []int       `json:"menu_highlight_word_offsets"`
	IntroCopper, FameCopper, InterludeCopper [][2]uint16
	FontOffset                               int      `json:"interlude_font_graphics_offset"`
	FontSource                               string   `json:"interlude_font_source"`
	PaddleDeathSprites                       []string `json:"paddle_death_sprites"`
}

// Presentation restores disk presentation data with newly written Go logic.
// Color-register pairs are data; the original instructions are never loaded.
type Presentation struct {
	Table           scores.Table
	ScoreError      error
	Name            NameEntry
	Intro           IntroSequence
	Interlude       Interlude
	Credits         [10]string
	directory       string
	data            presentationData
	menuIndices     []byte
	menuPixels      []byte
	menuImage       *ebiten.Image
	fameGlyphs      [40]*ebiten.Image
	interludeGlyphs [43][]byte
	interludePixels []byte
	interludeImage  *ebiten.Image
	clues           [7][]string
	introPixels     []byte
	introSource     []byte
	introImage      *ebiten.Image
}

// NewPresentation decodes source names, palette animation data, and clue text.
// ScoreError reports a local score-load failure while retaining disk defaults.
func NewPresentation(directory string) (*Presentation, error) {
	menu, err := assets.Read("original/menu.art")
	if err != nil {
		return nil, err
	}
	table, err := scores.DecodeOriginal(menu)
	if err != nil {
		return nil, err
	}
	p := &Presentation{Table: table, directory: directory, menuIndices: planarIndices(menu, 320, 200), menuPixels: make([]byte, 320*200*4), interludePixels: make([]byte, 320*200*4)}
	metadata, err := assets.Read("presentation.json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(metadata, &p.data); err != nil {
		return nil, err
	}
	// Explicit tags handle the original snake-case metadata names.
	var lists struct {
		Intro     [][2]uint16 `json:"intro_copper"`
		Fame      [][2]uint16 `json:"fame_copper"`
		Interlude [][2]uint16 `json:"interlude_copper"`
	}
	if err := json.Unmarshal(metadata, &lists); err != nil {
		return nil, err
	}
	p.data.IntroCopper, p.data.FameCopper, p.data.InterludeCopper = lists.Intro, lists.Fame, lists.Interlude
	if len(p.data.MenuCycle) != 120 || len(p.data.MenuCopper) < 100 || len(p.data.InterludeCopper) != 369 || len(p.data.HighlightOffsets) != 3 || len(p.data.PaddleDeathSprites) != 6 {
		return nil, fmt.Errorf("incomplete original presentation palette data")
	}
	copyright, err := assets.Read("original/copyrigh.txt")
	if err != nil {
		return nil, err
	}
	if len(copyright) < 240 {
		return nil, fmt.Errorf("incomplete original credits table")
	}
	for row := range p.Credits {
		var line strings.Builder
		for _, code := range copyright[row*24 : (row+1)*24] {
			if code == 0 {
				line.WriteByte(' ')
			} else if code <= 26 {
				line.WriteByte('A' + code - 1)
			} else {
				return nil, fmt.Errorf("invalid original credit glyph")
			}
		}
		p.Credits[row] = line.String()
	}
	textAt := func(address int) ([]string, error) {
		offset := address - 0x8826
		if offset < 0 || offset >= len(copyright) {
			return nil, fmt.Errorf("invalid original text address")
		}
		end := bytes.IndexByte(copyright[offset:], 255)
		if end < 0 {
			return nil, fmt.Errorf("unterminated original presentation text")
		}
		return strings.Split(string(copyright[offset:offset+end]), "\r"), nil
	}
	p.Intro.Lines, err = textAt(0x8916)
	if err != nil {
		return nil, err
	}
	for i, address := range []int{0x8df2, 0x8f1e, 0x904a, 0x9176, 0x92a2, 0x93ce, 0x94fa} {
		p.clues[i], err = textAt(address)
		if err != nil {
			return nil, err
		}
	}
	intro, err := assets.Image("images/intro.png")
	if err != nil {
		return nil, err
	}
	p.introSource = rgbaPixels(intro)
	p.introPixels = make([]byte, len(p.introSource))
	p.introImage = ebiten.NewImage(320, 200)
	p.menuImage = ebiten.NewImage(320, 200)
	p.interludeImage = ebiten.NewImage(320, 200)
	fame, err := assets.Read("original/fame.art")
	if err != nil {
		return nil, err
	}
	famePalette := copperRows(p.data.FameCopper, 200)[68]
	for glyph := range p.fameGlyphs {
		indices := planarIndices(fame[32000+glyph*32:32000+(glyph+1)*32], 8, 8)
		pixels := make([]byte, 8*8*4)
		for index, value := range indices {
			if value != 0 {
				writeRGB12(pixels, index*4, famePalette[value])
			}
		}
		p.fameGlyphs[glyph] = ebiten.NewImage(8, 8)
		p.fameGlyphs[glyph].WritePixels(pixels)
	}
	// Combat replaces the graphics buffer with final.bmp before using this
	// monochrome font. The same address in zz_3.bmp contains unrelated pixels.
	graphics, err := assets.Read(p.data.FontSource)
	if err != nil {
		return nil, err
	}
	if p.data.FontOffset < 0 || p.data.FontOffset+43*16 != len(graphics) {
		return nil, fmt.Errorf("invalid original interlude font bank: expected 43 final 8 by 16 glyphs")
	}
	for glyph := range p.interludeGlyphs {
		p.interludeGlyphs[glyph] = append([]byte(nil), graphics[p.data.FontOffset+glyph*16:p.data.FontOffset+(glyph+1)*16]...)
	}
	p.Table, p.ScoreError = scores.Load(directory, table)
	return p, nil
}

// SkipIntro permits explicit editor, round-selection, and automated run starts.
func (p *Presentation) SkipIntro() { p.Intro.Phase = IntroFinished }

// BeginName requests a name only when the completed run enters the top ten.
func (p *Presentation) BeginName(score int) bool {
	rank := p.Table.Rank(score)
	if rank < 0 {
		return false
	}
	p.Name = NameEntry{Active: true, Score: score, Rank: rank}
	return true
}

// FinishName inserts the completed run once and persists its chosen name.
func (p *Presentation) FinishName() error {
	if !p.Name.Active {
		return nil
	}
	p.Table.Insert(p.Name.Name, p.Name.Score)
	p.Name.Active = false
	return p.Table.Save(p.directory)
}

// BeginInterlude selects the exact message printed by the original combat path.
func (p *Presentation) BeginInterlude(combat int, lost bool) {
	index := max(0, min(5, combat-1))
	if lost {
		index = 6
	}
	p.Interlude = Interlude{Active: true, Combat: combat, Lines: p.clues[index]}
}

// UpdateInterlude preserves a fire edge for continuing to the next round.
func (p *Presentation) UpdateInterlude(fire bool) {
	if !p.Interlude.Active {
		return
	}
	p.Interlude.Ticks++
	if fire && p.Interlude.Ticks > 15 {
		p.Interlude.Active = false
	}
}

// DrawIntro reproduces the original bottom-up folded-line image reveal.
func (p *Presentation) DrawIntro(g *Graphics, screen *ebiten.Image) {
	if p.Intro.Phase == IntroReveal {
		row := max(0, 199-p.Intro.Ticks)
		for y := 0; y < 200; y++ {
			source := max(y, row)
			copy(p.introPixels[y*1280:(y+1)*1280], p.introSource[source*1280:(source+1)*1280])
		}
		p.introImage.WritePixels(p.introPixels)
		g.draw(screen, p.introImage, 0, 0, 1, 1)
		return
	}
	if p.Intro.Phase == IntroWait {
		g.Image(screen, "images/intro.png", 0, 0)
		return
	}
	if p.Intro.Phase == IntroCredits {
		// The original scroll retains the outgoing ship image above the text.
		if p.Intro.Ticks < 200 {
			g.draw(screen, g.images["images/intro.png"].SubImage(image.Rect(0, p.Intro.Ticks, 320, 200)).(*ebiten.Image), 0, 0, 1, 1)
		}
		for line, text := range p.Intro.Lines {
			y := 200 - p.Intro.Ticks + line*8
			if y > -8 && y < 200 {
				for column, ch := range strings.ToUpper(text) {
					index := -1
					if ch >= 'A' && ch <= 'Z' {
						index = int(ch - 'A')
					}
					if ch >= '0' && ch <= '9' {
						index = int(ch-'0') + 26
					}
					if index < 0 || 32+column*8 >= 320 {
						continue
					}
					glyph := g.font[index]
					bounds := glyph.Bounds()
					crop := image.Rect(bounds.Min.X, bounds.Min.Y+max(0, -y), bounds.Max.X, bounds.Min.Y+min(8, 200-y))
					g.draw(screen, glyph.SubImage(crop).(*ebiten.Image), float64(32+column*8), float64(max(0, y)), 1, 1)
				}
			}
		}
	}
}

// DrawMenu rotates the original 120-color band and original option highlight.
func (p *Presentation) DrawMenu(g *Graphics, screen *ebiten.Image, tick int, selectedScores bool) {
	copper := append([][2]uint16(nil), p.data.MenuCopper...)
	for row := 0; row < 32; row++ {
		copper[30+row*2][1] = p.data.MenuCycle[(tick+row)%len(p.data.MenuCycle)]
	}
	if selectedScores {
		for i, offset := range p.data.HighlightOffsets[:2] {
			for colorIndex := 0; colorIndex < 8; colorIndex++ {
				index := (offset + colorIndex*4 - 2) / 4
				value := copper[index][1]
				if i == 0 {
					value = (value >> 1) & 0x777
				} else {
					value = (value << 1) + 1
				}
				copper[index][1] = value
			}
		}
	}
	rows := copperRows(copper, 200)
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			position := y*320 + x
			writeRGB12(p.menuPixels, position*4, rows[y][p.menuIndices[position]])
		}
	}
	p.menuImage.WritePixels(p.menuPixels)
	g.draw(screen, p.menuImage, 0, 0, 1, 1)
}

func (p *Presentation) fameText(g *Graphics, screen *ebiten.Image, text string, x, y float64) {
	for _, ch := range strings.ToUpper(text) {
		index := -1
		if ch >= 'A' && ch <= 'Z' {
			index = int(ch - 'A')
		}
		if ch >= '0' && ch <= '9' {
			index = int(ch-'0') + 30
		}
		if index >= 0 {
			g.draw(screen, p.fameGlyphs[index], x, y, 1, 1)
		}
		x += 8
	}
}

// DrawHall uses the original fame font and its original 24-cell row positions.
func (p *Presentation) DrawHall(g *Graphics, screen *ebiten.Image, tick int) {
	g.Image(screen, "images/fame.png", 0, 0)
	displayed := p.Table
	if p.Name.Active {
		displayed.Insert("PLAYER", p.Name.Score)
	}
	for rank, entry := range displayed.Entries {
		name, score := entry.Name, entry.Score
		if p.Name.Active && rank == p.Name.Rank {
			name, score = p.Name.Name, p.Name.Score
		}
		p.fameText(g, screen, name, 64, float64(68+rank*8))
		number := fmt.Sprintf("%d", score)
		p.fameText(g, screen, number, float64(256-len(number)*8), float64(68+rank*8))
	}
	if !p.Name.Active {
		return
	}
	if tick%30 < 15 && len(p.Name.Name) < scores.NameLength {
		g.Image(screen, "sprites/sprite-00.png", float64(64+len(p.Name.Name)*8), float64(68+p.Name.Rank*8))
	}
	for row := 0; row < 3; row++ {
		for column := 0; column < 12; column++ {
			p.fameText(g, screen, nameKeys[row*12+column:row*12+column+1], float64(68+column*16), float64(152+row*11))
		}
	}
	g.Text(screen, "ERASE    SPACE    OK", 64, 188)
}

// DrawCredits reproduces the copyright rows available from the original menu.
func (p *Presentation) DrawCredits(g *Graphics, screen *ebiten.Image) {
	g.Image(screen, "images/fame.png", 0, 0)
	for row, text := range p.Credits {
		p.fameText(g, screen, text, 64, float64(68+row*8))
	}
}

// DrawLevelBanner uses the original zero-based label and 16 by 17 digits.
func (p *Presentation) DrawLevelBanner(g *Graphics, screen *ebiten.Image, level int) {
	level = max(0, min(99, level))
	g.Clear(screen, 80, 79, 144, 40)
	g.Image(screen, "sprites/level-label.png", 96, 88)
	g.Image(screen, fmt.Sprintf("sprites/level-digit-%d.png", level/10), 176, 88)
	g.Image(screen, fmt.Sprintf("sprites/level-digit-%d.png", level%10), 192, 88)
}

// DrawPaddleDeath follows the original width-slot decrement and sprite table.
func (p *Presentation) DrawPaddleDeath(g *Graphics, screen *ebiten.Image, w *game.World) {
	slot := w.DeathSlot
	if slot < 0 {
		return
	}
	x, y := w.Paddle.X, w.Paddle.Y-w.Paddle.H/2
	if slot >= 6 {
		bank := "normal"
		if w.Active(game.Sticky) {
			bank = "magnet"
		}
		if w.Active(game.Laser) || w.Active(game.Rocket) || w.Active(game.EnhancedCannon) {
			bank = "weapon"
		}
		width := game.PaddleWidths[min(24, slot)-6]
		g.Image(screen, fmt.Sprintf("sprites/paddle-%s-%d.png", bank, slot), x-width/2, y)
	} else if slot < len(p.data.PaddleDeathSprites) {
		g.Image(screen, p.data.PaddleDeathSprites[slot], x, y)
	}
}

// DrawInterlude reproduces the monochrome disk font and 180-color rotation.
func (p *Presentation) DrawInterlude(g *Graphics, screen *ebiten.Image) {
	copper := append([][2]uint16(nil), p.data.InterludeCopper...)
	for i := 0; i < 180; i++ {
		copper[9+i*2][1] = p.data.InterludeCopper[9+((i+p.Interlude.Ticks*2)%180)*2][1]
	}
	rows := copperRows(copper, 200)
	clear(p.interludePixels)
	for line, text := range p.Interlude.Lines {
		for column, ch := range text {
			glyph := interludeGlyph(ch)
			if glyph < 0 {
				continue
			}
			for y, bits := range p.interludeGlyphs[glyph] {
				py := line*20 + y
				if py >= 200 {
					continue
				}
				for x := 0; x < 8; x++ {
					px := column*8 + x
					if px < 320 && bits&(128>>x) != 0 {
						writeRGB12(p.interludePixels, (py*320+px)*4, rows[py][1])
					}
				}
			}
		}
	}
	p.interludeImage.WritePixels(p.interludePixels)
	g.draw(screen, p.interludeImage, 0, 0, 1, 1)
}

func interludeGlyph(ch rune) int {
	if ch >= 'a' && ch <= 'z' {
		return int(ch - 'a')
	}
	if ch >= 'A' && ch <= 'Z' {
		return int(ch - 'A')
	}
	if ch >= '0' && ch <= '9' {
		return int(ch-'0') + 26
	}
	switch ch {
	case '(':
		return 36
	case ')':
		return 37
	case '?':
		return 38
	case '!':
		return 39
	case '.':
		return 40
	case '\'':
		return 41
	case ':':
		return 42
	}
	return -1
}

func planarIndices(data []byte, width, height int) []byte {
	indices := make([]byte, width*height)
	stride, size := width/8, width/8*height
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			position, bit := y*stride+x/8, 7-x%8
			for plane := 0; plane < 4; plane++ {
				indices[y*width+x] |= ((data[plane*size+position] >> bit) & 1) << plane
			}
		}
	}
	return indices
}

func copperRows(copper [][2]uint16, height int) [][16]uint16 {
	type change struct {
		index int
		value uint16
	}
	changes := make(map[int][]change)
	line, wrapped, previous := 0, 0, 0
	for _, pair := range copper {
		register, value := pair[0], pair[1]
		if register == 0xffff && value == 0xfffe {
			break
		}
		if register&1 != 0 {
			beam := int(register >> 8)
			if beam < previous {
				wrapped = 256
			}
			previous = beam
			line = max(0, beam+wrapped-0x3e)
		} else if register >= 0x180 && register <= 0x19e {
			changes[line] = append(changes[line], change{int(register-0x180) / 2, value})
		}
	}
	rows := make([][16]uint16, height)
	var colors [16]uint16
	for y := range rows {
		for _, item := range changes[y] {
			colors[item.index] = item.value
		}
		rows[y] = colors
	}
	return rows
}

func writeRGB12(pixels []byte, position int, value uint16) {
	pixels[position] = byte((value>>8)&15) * 17
	pixels[position+1] = byte((value>>4)&15) * 17
	pixels[position+2] = byte(value&15) * 17
	pixels[position+3] = 255
}

func rgbaPixels(source image.Image) []byte {
	bounds := source.Bounds()
	result := make([]byte, bounds.Dx()*bounds.Dy()*4)
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			colorValue := color.NRGBAModel.Convert(source.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			offset := (y*bounds.Dx() + x) * 4
			result[offset], result[offset+1], result[offset+2], result[offset+3] = colorValue.R, colorValue.G, colorValue.B, colorValue.A
		}
	}
	return result
}
