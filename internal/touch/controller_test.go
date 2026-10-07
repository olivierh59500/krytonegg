package touch

import "testing"

var (
	playingField = Rect{X: 0, Y: 0, W: 320, H: 200}
	playButtons  = []Button{
		{Action: Pause, Bounds: Rect{X: 340, Y: 0, W: 40, H: 40}},
		{Action: Fire, Bounds: Rect{X: 340, Y: 60, W: 40, H: 40}},
	}
)

func TestDragAnchorsWithoutJumpAndPreservesVerticalDelta(t *testing.T) {
	var controller Controller
	frame := controller.Update([]Sample{{ID: 12, X: 40, Y: 70, Pressed: true}}, playButtons, playingField, ModeDrag, 160, 187)
	if !frame.Started || !frame.Move || frame.X != 160 || frame.Y != 187 {
		t.Fatalf("first contact moved the target: %+v", frame)
	}
	frame = controller.Update([]Sample{{ID: 12, X: 71.5, Y: 25.25}}, playButtons, playingField, ModeDrag, 160, 187)
	if frame.Started || !frame.Move || frame.X != 191.5 || frame.Y != 142.25 {
		t.Fatalf("finger deltas were not preserved: %+v", frame)
	}
	// Later target values must not accumulate the same displacement repeatedly.
	frame = controller.Update([]Sample{{ID: 12, X: 71.5, Y: 25.25}}, playButtons, playingField, ModeDrag, 191.5, 142.25)
	if frame.X != 191.5 || frame.Y != 142.25 {
		t.Fatalf("stationary finger continued moving the target: %+v", frame)
	}
}

func TestReorderedFingersMoveAndFireIndependently(t *testing.T) {
	var controller Controller
	controller.Update([]Sample{{ID: 42, X: 100, Y: 100, Pressed: true}}, playButtons, playingField, ModeDrag, 160, 187)
	frame := controller.Update([]Sample{
		{ID: 7, X: 360, Y: 80, Pressed: true},
		{ID: 42, X: 120, Y: 100},
	}, playButtons, playingField, ModeDrag, 160, 187)
	if !frame.Move || frame.X != 180 || !frame.Launch || !frame.Fire {
		t.Fatalf("simultaneous steering and fire failed: %+v", frame)
	}
	if len(frame.Actions) != 1 || frame.Actions[0] != Fire {
		t.Fatalf("fire press did not produce one action: %v", frame.Actions)
	}
	frame = controller.Update([]Sample{
		{ID: 42, X: 140, Y: 90},
		{ID: 7, X: 360, Y: 80},
	}, playButtons, playingField, ModeDrag, 180, 187)
	if frame.X != 200 || frame.Y != 177 || !frame.Fire || frame.Launch || len(frame.Actions) != 0 {
		t.Fatalf("held fire or reordered steering produced incorrect controls: %+v", frame)
	}
}

func TestCapturedDragCrossesButtonsWithoutPressingThem(t *testing.T) {
	var controller Controller
	controller.Update([]Sample{{ID: 0, X: 100, Y: 100, Pressed: true}}, playButtons, playingField, ModeDrag, 160, 187)
	frame := controller.Update([]Sample{{ID: 0, X: 360, Y: 20}}, playButtons, playingField, ModeDrag, 160, 187)
	if !frame.Move || frame.X != 420 || len(frame.Actions) != 0 {
		t.Fatalf("crossing the pause button changed gesture ownership: %+v", frame)
	}
	frame = controller.Update([]Sample{{ID: 0, X: 360, Y: 80}}, playButtons, playingField, ModeDrag, 320, 107)
	if !frame.Move || frame.Fire || frame.Launch || len(frame.Actions) != 0 {
		t.Fatalf("crossing the fire button activated it: %+v", frame)
	}
}

func TestButtonFingerCannotSteerOrPressAnotherButton(t *testing.T) {
	var controller Controller
	frame := controller.Update([]Sample{{ID: 1, X: 360, Y: 80, Pressed: true}}, playButtons, playingField, ModeDrag, 160, 187)
	if frame.Move || !frame.Launch || !frame.Fire {
		t.Fatalf("initial fire gesture was not isolated from steering: %+v", frame)
	}
	frame = controller.Update([]Sample{{ID: 1, X: 80, Y: 100}}, playButtons, playingField, ModeDrag, 160, 187)
	if frame.Move || frame.Fire || frame.Launch || len(frame.Actions) != 0 {
		t.Fatalf("button finger took ownership of the field: %+v", frame)
	}
	frame = controller.Update([]Sample{{ID: 1, X: 360, Y: 20}}, playButtons, playingField, ModeDrag, 160, 187)
	if frame.Fire || len(frame.Actions) != 0 {
		t.Fatalf("button finger pressed another button while held: %+v", frame)
	}
	frame = controller.Update([]Sample{
		{ID: 1, X: 360, Y: 80},
		{ID: 2, X: 70, Y: 100, Pressed: true},
	}, playButtons, playingField, ModeDrag, 160, 187)
	if !frame.Move || frame.X != 160 || !frame.Fire || frame.Launch || len(frame.Actions) != 0 {
		t.Fatalf("fire did not resume independently of a new steering finger: %+v", frame)
	}
}

func TestReleaseRequiresFreshPressBeforeTakingOver(t *testing.T) {
	var controller Controller
	controller.Update([]Sample{
		{ID: 1, X: 100, Y: 100, Pressed: true},
		{ID: 2, X: 250, Y: 100, Pressed: true},
	}, playButtons, playingField, ModeDrag, 160, 187)
	frame := controller.Update([]Sample{{ID: 2, X: 260, Y: 100}}, playButtons, playingField, ModeDrag, 180, 187)
	if frame.Move {
		t.Fatalf("already held finger took over after release: %+v", frame)
	}
	controller.Update(nil, playButtons, playingField, ModeDrag, 180, 187)
	frame = controller.Update([]Sample{{ID: 2, X: 260, Y: 100, Pressed: true}}, playButtons, playingField, ModeDrag, 180, 187)
	if !frame.Move || frame.X != 180 || frame.Y != 187 {
		t.Fatalf("fresh press did not anchor to the current target: %+v", frame)
	}
}

func TestButtonActionsOccurOnlyOnPress(t *testing.T) {
	var controller Controller
	frame := controller.Update([]Sample{{ID: 3, X: 360, Y: 20, Pressed: true}}, playButtons, playingField, ModeDrag, 160, 187)
	if len(frame.Actions) != 1 || frame.Actions[0] != Pause {
		t.Fatalf("pause press failed: %+v", frame)
	}
	frame = controller.Update([]Sample{{ID: 3, X: 360, Y: 20}}, playButtons, playingField, ModeDrag, 160, 187)
	if len(frame.Actions) != 0 {
		t.Fatalf("held pause repeated its action: %v", frame.Actions)
	}
	controller.Update(nil, playButtons, playingField, ModeDrag, 160, 187)
	frame = controller.Update([]Sample{{ID: 3, X: 360, Y: 20, Pressed: true}}, playButtons, playingField, ModeDrag, 160, 187)
	if len(frame.Actions) != 1 || frame.Actions[0] != Pause {
		t.Fatalf("released button could not be pressed again: %+v", frame)
	}
}

func TestReusedPointerIDReanchorsOnFreshPress(t *testing.T) {
	var controller Controller
	controller.Update([]Sample{{ID: 1, X: 100, Y: 100, Pressed: true}}, playButtons, playingField, ModeDrag, 160, 187)
	controller.Update([]Sample{{ID: 1, X: 140, Y: 100}}, playButtons, playingField, ModeDrag, 160, 187)
	// A release and a new press can both happen between two game updates.
	frame := controller.Update([]Sample{{ID: 1, X: 250, Y: 60, Pressed: true}}, playButtons, playingField, ModeDrag, 200, 187)
	if !frame.Started || !frame.Move || frame.X != 200 || frame.Y != 187 {
		t.Fatalf("reused pointer ID kept the previous drag anchor: %+v", frame)
	}
	frame = controller.Update([]Sample{{ID: 1, X: 360, Y: 80, Pressed: true}}, playButtons, playingField, ModeDrag, 200, 187)
	if frame.Started || frame.Move || !frame.Fire || !frame.Launch {
		t.Fatalf("reused pointer ID could not start a separate button gesture: %+v", frame)
	}
}

func TestPaintingUsesAbsoluteCoordinatesAndKeepsItsFinger(t *testing.T) {
	var controller Controller
	buttons := append(append([]Button(nil), playButtons...), Button{Action: Save, Bounds: Rect{X: 340, Y: 120, W: 40, H: 40}})
	frame := controller.Update([]Sample{{ID: 6, X: 125, Y: 65, Pressed: true}}, buttons, playingField, ModePaint, 160, 187)
	if !frame.Painting || frame.PaintX != 125 || frame.PaintY != 65 || frame.Move {
		t.Fatalf("paint contact did not use absolute coordinates: %+v", frame)
	}
	frame = controller.Update([]Sample{{ID: 6, X: 360, Y: 20}}, buttons, playingField, ModePaint, 160, 187)
	if frame.Painting || len(frame.Actions) != 0 {
		t.Fatalf("painting outside the field activated a button: %+v", frame)
	}
	frame = controller.Update([]Sample{
		{ID: 9, X: 360, Y: 140, Pressed: true},
		{ID: 6, X: 145, Y: 75},
	}, buttons, playingField, ModePaint, 160, 187)
	if !frame.Painting || frame.PaintX != 145 || frame.PaintY != 75 || len(frame.Actions) != 1 || frame.Actions[0] != Save {
		t.Fatalf("painting and a second-finger button interfered: %+v", frame)
	}
}

func TestMenuAndModeChangesRequireNewFieldGestures(t *testing.T) {
	var controller Controller
	buttons := []Button{{Action: Play, Bounds: Rect{X: 100, Y: 120, W: 100, H: 30}}}
	frame := controller.Update([]Sample{{ID: 5, X: 100, Y: 100, Pressed: true}}, buttons, playingField, ModeMenu, 160, 187)
	if frame.Move || frame.Painting {
		t.Fatalf("menu captured a field finger: %+v", frame)
	}
	frame = controller.Update([]Sample{
		{ID: 5, X: 110, Y: 100},
		{ID: 8, X: 140, Y: 135, Pressed: true},
	}, buttons, playingField, ModeMenu, 160, 187)
	if len(frame.Actions) != 1 || frame.Actions[0] != Play {
		t.Fatalf("menu play button failed: %+v", frame)
	}
	frame = controller.Update([]Sample{{ID: 5, X: 110, Y: 100}}, playButtons, playingField, ModeDrag, 160, 187)
	if frame.Move {
		t.Fatalf("menu finger began steering without a fresh press: %+v", frame)
	}
	frame = controller.Update([]Sample{{ID: 2, X: 110, Y: 100, Pressed: true}}, playButtons, playingField, ModeDrag, 160, 187)
	if !frame.Move {
		t.Fatal("fresh field finger was not captured")
	}
	frame = controller.Update([]Sample{{ID: 2, X: 120, Y: 110}}, playButtons, playingField, ModePaint, 160, 187)
	if frame.Move || frame.Painting {
		t.Fatalf("mode change reused the previous gesture: %+v", frame)
	}
}

func TestGestureStartingOutsideFieldCannotTakeOverBySlidingInside(t *testing.T) {
	var controller Controller
	controller.Update([]Sample{{ID: 4, X: -1, Y: 90, Pressed: true}}, playButtons, playingField, ModeDrag, 160, 187)
	frame := controller.Update([]Sample{{ID: 4, X: 40, Y: 90}}, playButtons, playingField, ModeDrag, 160, 187)
	if frame.Move {
		t.Fatalf("outside gesture was captured after entering the field: %+v", frame)
	}
}

func TestHiddenFireButtonStopsHeldFireWithoutPressingReplacement(t *testing.T) {
	var controller Controller
	controller.Update([]Sample{{ID: 1, X: 360, Y: 80, Pressed: true}}, playButtons, playingField, ModeDrag, 160, 187)
	buttons := []Button{{Action: Menu, Bounds: playButtons[1].Bounds}}
	frame := controller.Update([]Sample{{ID: 1, X: 360, Y: 80}}, buttons, playingField, ModeMenu, 160, 187)
	if frame.Fire || frame.Launch || len(frame.Actions) != 0 {
		t.Fatalf("held fire activated a replacement control: %+v", frame)
	}
}

func TestButtonWinsOverOverlappingField(t *testing.T) {
	var controller Controller
	buttons := []Button{{Action: Editor, Bounds: Rect{X: 20, Y: 20, W: 40, H: 40}}}
	frame := controller.Update([]Sample{{ID: 1, X: 30, Y: 30, Pressed: true}}, buttons, playingField, ModeDrag, 160, 187)
	if frame.Move || len(frame.Actions) != 1 || frame.Actions[0] != Editor {
		t.Fatalf("overlapping field consumed the button press: %+v", frame)
	}
}

func TestRectangleEdgesDoNotOverlap(t *testing.T) {
	left := Rect{X: 10, Y: 20, W: 30, H: 40}
	right := Rect{X: 40, Y: 20, W: 30, H: 40}
	if !left.Contains(10, 20) || left.Contains(40, 30) || left.Contains(20, 60) || !right.Contains(40, 30) {
		t.Fatal("rectangle boundaries overlap or exclude their starting corner")
	}
	if (Rect{}).Contains(0, 0) || (Rect{W: -1, H: 2}).Contains(-1, 1) {
		t.Fatal("empty rectangles captured a finger")
	}
}
