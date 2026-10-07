// Package scores preserves the original ten-entry named hall of fame.
package scores

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	Capacity = 10
	// NameLength follows the original entry loop at code address 0x7fe.
	NameLength = 16
)

// Entry stores a name and the game's unsigned sixteen-bit score.
type Entry struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

// Table keeps entries in descending score order; equal scores stay stable.
type Table struct {
	Entries [Capacity]Entry `json:"entries"`
}

// DecodeOriginal reads the names and scores appended to the disk's menu.art.
// Its ten display rows contain 24 character codes; scores follow at 32248.
func DecodeOriginal(menu []byte) (Table, error) {
	var table Table
	if len(menu) < 32268 {
		return table, fmt.Errorf("original menu must contain the complete score table")
	}
	for row := range table.Entries {
		var name strings.Builder
		for _, code := range menu[32000+row*24 : 32000+row*24+20] {
			if code == 0 {
				name.WriteByte(' ')
			} else if code <= 26 {
				name.WriteByte('A' + code - 1)
			} else {
				return Table{}, fmt.Errorf("invalid original name code %d", code)
			}
		}
		table.Entries[row] = Entry{strings.TrimSpace(name.String()), int(binary.BigEndian.Uint16(menu[32248+row*2:]))}
	}
	return table, nil
}

// Best returns the displayed high score.
func (t Table) Best() int { return t.Entries[0].Score }

// Rank returns a qualifying insertion position, or -1. The original CMP.W/BGT
// at 0x60c compares signed words, although stored scores and decimal rendering
// are unsigned. Scores above 32767 therefore retain that source limitation.
func (t Table) Rank(score int) int {
	if score < 0 || score > 65535 {
		return -1
	}
	for rank, entry := range t.Entries {
		if int16(score) > int16(entry.Score) {
			return rank
		}
	}
	return -1
}

// CleanName restricts new entries to characters present in the source font.
// Existing original defaults may be longer than the interactive entry limit.
func CleanName(name string) string {
	var result strings.Builder
	for _, ch := range strings.ToUpper(name) {
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == ' ' {
			result.WriteRune(ch)
		}
		if result.Len() == NameLength {
			break
		}
	}
	return strings.TrimSpace(result.String())
}

// Insert registers a completed run and returns its rank, or -1.
func (t *Table) Insert(name string, score int) int {
	rank := t.Rank(score)
	if rank < 0 {
		return -1
	}
	name = CleanName(name)
	if name == "" {
		name = "PLAYER"
	}
	copy(t.Entries[rank+1:], t.Entries[rank:Capacity-1])
	t.Entries[rank] = Entry{Name: name, Score: score}
	return rank
}

// Load restores the named table or migrates the former single-score file.
// Missing files return the unchanged disk defaults. Malformed files surface an
// error so callers can report persistence failures without losing defaults.
func Load(directory string, defaults Table) (Table, error) {
	data, err := os.ReadFile(filepath.Join(directory, "halloffame.json"))
	if errors.Is(err, os.ErrNotExist) {
		legacy, legacyErr := os.ReadFile(filepath.Join(directory, "highscore.json"))
		if errors.Is(legacyErr, os.ErrNotExist) {
			return defaults, nil
		}
		if legacyErr != nil {
			return defaults, legacyErr
		}
		var old struct {
			Score int `json:"score"`
		}
		if err := json.Unmarshal(legacy, &old); err != nil {
			return defaults, err
		}
		defaults.Insert("PLAYER", old.Score)
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	var stored struct {
		Version int     `json:"version"`
		Entries []Entry `json:"entries"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return defaults, err
	}
	if stored.Version != 1 || len(stored.Entries) != Capacity {
		return defaults, fmt.Errorf("unsupported or incomplete hall of fame")
	}
	for _, entry := range stored.Entries {
		if entry.Score < 0 || entry.Score > 65535 || entry.Name == "" || len(entry.Name) > 20 {
			return defaults, fmt.Errorf("invalid hall of fame entry")
		}
		for _, ch := range entry.Name {
			if !((ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == ' ') {
				return defaults, fmt.Errorf("invalid hall of fame name")
			}
		}
	}
	sort.SliceStable(stored.Entries, func(i, j int) bool {
		return int16(stored.Entries[i].Score) > int16(stored.Entries[j].Score)
	})
	copy(defaults.Entries[:], stored.Entries)
	return defaults, nil
}

// Save replaces only the local named score file, using an atomic rename.
func (t Table) Save(directory string) error {
	if directory == "" {
		directory = "."
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(struct {
		Version int             `json:"version"`
		Entries [Capacity]Entry `json:"entries"`
	}{1, t.Entries}, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".halloffame-*.json")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(path, filepath.Join(directory, "halloffame.json"))
}
