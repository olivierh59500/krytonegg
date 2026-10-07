package presentation

import (
	"strings"
	"testing"
)

func TestIntroRequiresSeparateFireEdgesForCreditsAndMenu(t *testing.T) {
	sequence := IntroSequence{Lines: []string{"", "TEST CREDITS", ""}}
	for i := 0; i < 200; i++ {
		sequence.Update(false)
	}
	if sequence.Phase != IntroWait {
		t.Fatal(sequence.Phase)
	}
	sequence.Update(true)
	if sequence.Phase != IntroCredits || sequence.Ticks != 0 {
		t.Fatal(sequence)
	}
	sequence.Update(false)
	if sequence.Ticks != 1 {
		t.Fatal(sequence)
	}
	sequence.Update(true)
	if sequence.Phase != IntroFinished {
		t.Fatal(sequence.Phase)
	}
}

func TestCreditsCompleteWithoutSkipping(t *testing.T) {
	sequence := IntroSequence{Phase: IntroCredits, Lines: []string{"TEST"}}
	for i := 0; i < 208; i++ {
		sequence.Update(false)
	}
	if sequence.Phase != IntroFinished {
		t.Fatal(sequence)
	}
}

func TestNameKeyboardAndTouchStayWithinSourceLimit(t *testing.T) {
	entry := NameEntry{Active: true}
	entry.Type("aé1 b!☃")
	if entry.Name != "A1 B" {
		t.Fatal(entry.Name)
	}
	entry.Backspace()
	if entry.Touch(68, 152) || entry.Name != "A1 A" {
		t.Fatal(entry.Name)
	}
	entry.Touch(252, 177)
	if entry.Name != "A1 A9" {
		t.Fatal(entry.Name)
	}
	entry.Touch(72, 190)
	if entry.Name != "A1 A" {
		t.Fatal(entry.Name)
	}
	entry.Type(strings.Repeat("X", 40))
	if len(entry.Name) != 16 || !entry.Touch(220, 190) {
		t.Fatal(entry)
	}
}

func TestClueInputWaitsForFreshContinue(t *testing.T) {
	p := Presentation{Interlude: Interlude{Active: true}}
	p.UpdateInterlude(true)
	if !p.Interlude.Active {
		t.Fatal("the winning fire edge skipped its clue")
	}
	for i := 0; i < 15; i++ {
		p.UpdateInterlude(false)
	}
	p.UpdateInterlude(true)
	if p.Interlude.Active {
		t.Fatal("fresh fire did not continue")
	}
}

func TestCopperAppliesChangesAcrossRasterWrap(t *testing.T) {
	rows := copperRows([][2]uint16{{0x180, 0x123}, {0xfe01, 0xfffe}, {0x182, 0x456}, {0x0101, 0xfffe}, {0x182, 0x789}, {0xffff, 0xfffe}}, 200)
	if rows[0][0] != 0x123 || rows[191][1] != 0 || rows[192][1] != 0x456 || rows[195][1] != 0x789 {
		t.Fatal("wrapped scanline palette was applied at the wrong row")
	}
}
