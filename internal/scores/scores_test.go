package scores

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func originalTable(t *testing.T) Table {
	t.Helper()
	menu := make([]byte, 32268)
	// These synthetic names exercise the disk encoding without checking any
	// original asset content into source control.
	names := []string{"FIRST PLAYER", "SECOND PLAYER", "THIRD PLAYER", "FOURTH PLAYER", "FIFTH PLAYER", "SIXTH PLAYER", "LONG ORIGINAL NAME", "EIGHTH PLAYER", "NINTH PLAYER", "LAST PLAYER"}
	values := []int{5000, 2500, 1000, 800, 700, 600, 500, 400, 300, 200}
	for i, name := range names {
		for j, ch := range name {
			if ch != ' ' {
				menu[32000+i*24+j] = byte(ch - 'A' + 1)
			}
		}
		binary.BigEndian.PutUint16(menu[32248+i*2:], uint16(values[i]))
	}
	table, err := DecodeOriginal(menu)
	if err != nil {
		t.Fatal(err)
	}
	if table.Entries[6].Name != "LONG ORIGINAL NAME" {
		t.Fatal(table.Entries[6])
	}
	return table
}

func TestQualifyingScoreDisplacesOnlyLowerEntries(t *testing.T) {
	table := originalTable(t)
	if table.Rank(200) != -1 || table.Rank(65535) != -1 || table.Rank(65536) != -1 {
		t.Fatal("invalid or tied cutoff qualified")
	}
	if rank := table.Insert("test player!", 800); rank != 4 {
		t.Fatal(rank)
	}
	if table.Entries[3].Name != "FOURTH PLAYER" || table.Entries[4].Name != "TEST PLAYER" || table.Entries[9].Score != 300 {
		t.Fatal(table)
	}
	if rank := table.Insert("12345678901234567890", 32767); rank != 0 || table.Entries[0].Name != "1234567890123456" {
		t.Fatal(table)
	}
}

func TestLegacyMigrationAndNamedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	defaults := originalTable(t)
	if err := os.WriteFile(filepath.Join(dir, "highscore.json"), []byte(`{"score":12345}`), 0600); err != nil {
		t.Fatal(err)
	}
	table, err := Load(dir, defaults)
	if err != nil || table.Entries[0] != (Entry{"PLAYER", 12345}) {
		t.Fatal(table, err)
	}
	table.Insert("ANDROID PLAYER", 20000)
	if err := table.Save(dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(dir, defaults)
	if err != nil || loaded != table {
		t.Fatal(loaded, err)
	}
}

func TestInvalidPersistenceKeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	defaults := originalTable(t)
	if err := os.WriteFile(filepath.Join(dir, "halloffame.json"), []byte(`{"version":1,"entries":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	table, err := Load(dir, defaults)
	if err == nil || table != defaults {
		t.Fatal(table, err)
	}
}
