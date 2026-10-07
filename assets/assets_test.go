package assets

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

// Every decoded level cell must retain the original word, including its bonus bits.
func TestCampaignJSONPreservesDiskWords(t *testing.T) {
	table, err := Read("original/level.tab")
	if err != nil {
		t.Fatal(err)
	}
	data, err := Read("levels.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Levels  [][]uint16 `json:"levels"`
		Enemies [][]uint16 `json:"enemy_choices"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	if len(metadata.Levels) != 60 || len(metadata.Enemies) != 60 {
		t.Fatal("missing original campaign data")
	}
	for round, cells := range metadata.Levels {
		if len(cells) != 288 || len(metadata.Enemies[round]) != 4 {
			t.Fatalf("incomplete round %d", round+1)
		}
		for cell, value := range cells {
			if value != binary.BigEndian.Uint16(table[(round*288+cell)*2:]) {
				t.Fatalf("changed source word in round %d, cell %d", round+1, cell)
			}
		}
	}
}

// The WAV representation must preserve every original signed Paula sample byte.
func TestEffectsPreservePaulaSamples(t *testing.T) {
	manifest, err := Read("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Sounds map[string]struct {
			Offset int `json:"offset"`
			Length int `json:"length_bytes"`
		} `json:"sounds"`
	}
	if err := json.Unmarshal(manifest, &metadata); err != nil {
		t.Fatal(err)
	}
	// Raw sprite/sample data is intentionally excluded from the embedded binary;
	// read the retained source reference only for this development provenance test.
	source, err := os.ReadFile("original/zz_3.bmp")
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata.Sounds) != 21 {
		t.Fatalf("expected 21 recovered original playback variants, got %d", len(metadata.Sounds))
	}
	for name, info := range metadata.Sounds {
		t.Run(name, func(t *testing.T) {
			wav, err := Read("audio/" + name + ".wav")
			if err != nil {
				t.Fatal(err)
			}
			if len(wav) < 44 || string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
				t.Fatal("invalid PCM WAV")
			}
			var samples []byte
			for cursor := 12; cursor+8 <= len(wav); {
				size := int(binary.LittleEndian.Uint32(wav[cursor+4 : cursor+8]))
				if cursor+8+size > len(wav) {
					t.Fatal("truncated WAV chunk")
				}
				if string(wav[cursor:cursor+4]) == "data" {
					samples = wav[cursor+8 : cursor+8+size]
					break
				}
				cursor += 8 + size + (size & 1)
			}
			if len(samples) != info.Length {
				t.Fatalf("wrong sample length: %d", len(samples))
			}
			original := append([]byte(nil), source[info.Offset:info.Offset+info.Length]...)
			for i := range original {
				original[i] ^= 0x80
			}
			if !bytes.Equal(samples, original) {
				t.Fatal("converted sample differs from original Paula bytes")
			}
		})
	}
}
