// Package touch translates independent touch gestures into game controls.
// It has no platform dependencies, so touch ownership can be tested headlessly.
package touch

// Action identifies a button without prescribing how the game handles it.
type Action int

const (
	None Action = iota
	Fire
	Pause
	Menu
	Sound
	Play
	Scores
	Editor
	Help
	RoundPrev
	RoundNext
	TilePrev
	TileNext
	Bonus
	Power
	Erase
	Save
	Load
	Test
	Back
)

// Mode selects how a gesture that starts in the playing field is interpreted.
type Mode int

const (
	ModeDrag Mode = iota
	ModePaint
	ModeMenu
)

// Rect uses native game coordinates and excludes its right and bottom edges.
type Rect struct{ X, Y, W, H float64 }

// Contains reports whether a point is inside a nonempty rectangle.
func (r Rect) Contains(x, y float64) bool {
	return r.W > 0 && r.H > 0 && x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H
}

// Sample describes an active finger. Pressed is true only on its first update.
type Sample struct {
	ID      int
	X, Y    float64
	Pressed bool
}

// Button assigns an action to a visible rectangle in native game coordinates.
type Button struct {
	Action Action
	Bounds Rect
}

// Frame contains the controls produced by the current active fingers.
// Started marks the first update of a newly captured field gesture. X and Y
// are valid when Move is true; PaintX and PaintY identify the painting finger.
// Actions and Launch are press edges, whereas Fire is held.
type Frame struct {
	Started        bool
	Move           bool
	X, Y           float64
	Painting       bool
	PaintX, PaintY float64
	Launch, Fire   bool
	Actions        []Action
}

type capture struct {
	id               int
	fingerX, fingerY float64
	targetX, targetY float64
}

// Controller owns each gesture until its finger is released. Its zero value
// is ready to use. Call Update once per game update with all active fingers.
type Controller struct {
	mode        Mode
	fieldFinger *capture
	buttons     map[int]Button
}

// Update anchors a new field gesture to the current target without moving it
// on contact. A captured finger keeps its role when crossing other controls,
// and a finger already held cannot take over when another finger is released.
func (c *Controller) Update(samples []Sample, buttons []Button, field Rect, mode Mode, targetX, targetY float64) Frame {
	if c.buttons == nil {
		c.buttons = make(map[int]Button)
	}
	active := make(map[int]Sample, len(samples))
	for _, sample := range samples {
		// The first occurrence also makes malformed duplicate samples harmless.
		if _, exists := active[sample.ID]; !exists {
			active[sample.ID] = sample
		}
	}
	if c.mode != mode {
		c.fieldFinger = nil
		c.mode = mode
	}
	if c.fieldFinger != nil {
		if _, exists := active[c.fieldFinger.id]; !exists {
			c.fieldFinger = nil
		}
	}
	for id := range c.buttons {
		if _, exists := active[id]; !exists {
			delete(c.buttons, id)
		}
	}

	var frame Frame
	current := make(map[int]struct{}, len(active))
	for _, sample := range samples {
		if _, handled := current[sample.ID]; handled {
			continue
		}
		current[sample.ID] = struct{}{}
		if !sample.Pressed {
			continue
		}
		// Android may reuse a pointer ID between updates. A fresh press is
		// authoritative even when the previous release was not sampled.
		delete(c.buttons, sample.ID)
		if c.fieldFinger != nil && c.fieldFinger.id == sample.ID {
			c.fieldFinger = nil
		}
		if button, hit := buttonAt(buttons, sample.X, sample.Y); hit {
			c.buttons[sample.ID] = button
			frame.Actions = append(frame.Actions, button.Action)
			if button.Action == Fire {
				frame.Launch = true
			}
			continue
		}
		if c.fieldFinger == nil && (mode == ModeDrag || mode == ModePaint) && field.Contains(sample.X, sample.Y) {
			c.fieldFinger = &capture{
				id: sample.ID, fingerX: sample.X, fingerY: sample.Y,
				targetX: targetX, targetY: targetY,
			}
			frame.Started = true
		}
	}

	if finger := c.fieldFinger; finger != nil {
		sample := active[finger.id]
		switch mode {
		case ModeDrag:
			frame.Move = true
			frame.X = finger.targetX + sample.X - finger.fingerX
			frame.Y = finger.targetY + sample.Y - finger.fingerY
		case ModePaint:
			frame.PaintX, frame.PaintY = sample.X, sample.Y
			frame.Painting = field.Contains(sample.X, sample.Y)
		}
	}
	for id, button := range c.buttons {
		if button.Action != Fire || !button.Bounds.Contains(active[id].X, active[id].Y) {
			continue
		}
		// Hiding a fire button disables a held gesture without changing its
		// ownership or turning it into a press on the replacement control.
		for _, visible := range buttons {
			if visible.Action == Fire {
				frame.Fire = true
				break
			}
		}
	}
	return frame
}

func buttonAt(buttons []Button, x, y float64) (Button, bool) {
	for _, button := range buttons {
		if button.Action != None && button.Bounds.Contains(x, y) {
			return button, true
		}
	}
	return Button{}, false
}
