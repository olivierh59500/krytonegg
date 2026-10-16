package game

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

const (
	LevelColumns       = 18
	LevelRows          = 16
	LevelByteSize      = LevelColumns * LevelRows * 2
	OriginalLevelCount = 60
)

// LoadLevels decodes the original big-endian 18 by 16 table without replacing
// campaign geometry. Sprite indices and embedded bonus bytes remain intact.
func LoadLevels(data []byte) ([]Level, error) {
	if len(data) == 0 || len(data)%LevelByteSize != 0 {
		return nil, fmt.Errorf("level table has %d bytes; expected complete %d-byte levels", len(data), LevelByteSize)
	}
	levels := make([]Level, len(data)/LevelByteSize)
	for i := range levels {
		level := &levels[i]
		level.Number, level.Name = i+1, fmt.Sprintf("ROUND %02d", i+1)
		for row := 0; row < LevelRows; row++ {
			for column := 0; column < LevelColumns; column++ {
				offset := i*LevelByteSize + (row*LevelColumns+column)*2
				code := binary.BigEndian.Uint16(data[offset : offset+2])
				kind := int(code & 0xff)
				if kind == 0 {
					continue
				}
				brick := Brick{X: FieldLeft + float64(column*16), Y: 24 + float64(row*8), W: 16, H: 8,
					Kind: kind, Code: code, HP: 1, Score: (kind & 15) / 2, Destructible: kind <= 0x60}
				if kind >= 0x11 && kind <= 0x30 {
					brick.HP = 1 + (kind-1)/16
				}
				if kind >= 0x41 && kind <= 0x60 {
					brick.HP = 1 + (kind-0x31)/16
				}
				if !brick.Destructible {
					brick.HP = -1
				}
				if kind >= 0x31 && kind <= 0x60 {
					brick.Bonus = BonusFromCode(uint8(code >> 8))
					brick.BonusPower = int(code>>8) & 3
				}
				level.Bricks = append(level.Bricks, brick)
			}
		}
	}
	return levels, nil
}

// LoadCampaign combines the unchanged level table with behavior metadata decoded
// from the original executable. No enemy types or round arrangements are invented.
func LoadCampaign(table, metadata []byte) ([]Level, error) {
	levels, err := LoadLevels(table)
	if err != nil {
		return nil, err
	}
	var data struct {
		EnemyChoices [][]uint16 `json:"enemy_choices"`
	}
	if err := json.Unmarshal(metadata, &data); err != nil {
		return nil, fmt.Errorf("decode campaign metadata: %w", err)
	}
	if len(data.EnemyChoices) != len(levels) {
		return nil, fmt.Errorf("campaign has %d rounds but %d enemy choices", len(levels), len(data.EnemyChoices))
	}
	for i := range levels {
		levels[i].EnemyChoices = append([]uint16(nil), data.EnemyChoices[i]...)
	}
	return levels, nil
}

// BonusFromCode maps recovered bonus identifiers to simulation effects. Unknown
// identifiers remain inert until their meaning can be established from the disk.
func BonusFromCode(code uint8) EffectKind {
	switch code >> 2 {
	case 1:
		return Grow
	case 2:
		return MouseSensitivity
	case 3:
		return ScoreBonus
	case 4:
		return Reverse
	case 6:
		return Sticky
	case 7:
		return ExtraLife
	case 8:
		return Multiball
	case 9:
		return LargeBall
	case 10:
		return Darkness
	case 11:
		return Fast
	case 12:
		return Slow
	case 13:
		return Autopilot
	case 14:
		return Flying
	case 15:
		return GhostPaddle
	case 16:
		return Shield
	case 17:
		return Laser
	case 18:
		return SuperBall
	case 19:
		return GhostBall
	case 20:
		return SecondPaddle
	case 21:
		return EnhancedCannon
	case 22:
		return RandomBonus
	case 23:
		return Shrink
	case 24:
		return WeaponMode1
	case 25:
		return WeaponMode2
	case 26:
		return WeaponMode4
	case 27:
		return WeaponMode8
	default:
		return NoEffect
	}
}
