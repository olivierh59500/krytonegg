# Original disk asset extraction

The input is `previous/Krypton Egg (1990)(HitSoft)[cr QTX].adf`, an 880 KiB AmigaDOS OFS disk. Its filesystem contains 17 files. `tools/extract_adf.py` follows the original OFS header and data sector chains, checks their bounds and sizes, and converts only data already present on this disk. The resulting game does not load the original executable, run original machine code, use an emulator, or need Python at runtime.

Run the reproducible conversion from the repository root:

```sh
python3 -m pip install Pillow
python3 tools/extract_adf.py
```

The converter reads `kry` into memory only to recover bitmap dimensions, palette registers, sprite pointers, enemy choices, and sound playback parameters. It does not place this executable in the shipped assets. The original ADF is retained locally as the reference source. All disk-derived files, including converted PNG/WAV data and metadata, are excluded from Git and its history. The extraction and recovery scripts remain versioned. Intermediate disassembly, if present under `.cache`, is excluded from Git.

## Asset inventory

- `assets/original/intro` and `halloffame` are the original four-channel, 31-sample-header tracker modules with the `M.K.` signature. Their byte contents are preserved.
- `assets/images/intro.png`, `menu.png`, and `fame.png` decode the original four-plane 320×200 images. The converter applies color register changes in the original copper lists at their raster positions. The menu's moving color effect and cursor highlight are runtime effects rather than new artwork.
- `assets/images/background-0.png` through `background-82.png` contain the 83 original repeating field textures, the original HUD, and the left and right walls. The source selects these textures from a sequential pointer cycle. The first texture is the green diamond pattern.
- `assets/sprites/brick-0.png` through `brick-255.png` preserve the complete 16×8 tile bank, indexed by the low byte of each original level cell. Tile zero is skipped during level rendering.
- `sprite-00` through `sprite-05` are ball sizes, `06` through `11` are sparks, `12` through `30` are normal paddle variants, `31` through `47` are weapon paddle variants, and `48` through `54` are paddle destruction frames. The images preserve the padded source width and the original transparency mask. Their visible bounds are recorded in `manifest.json`.
- `enemy-K-F.png` contains six enemy families with two or three frames per family. `enemy-death-0` through `20` preserve their death animation. `door-0` through `10` are the original opening door frames.
- `bonus-1` through `27` preserve the original drop graphics. `bonus-K-F` contains all frames addressed by their original animation flags. The first frame is also supplied under the shorter bonus name.
- `superball-0` through `5` and `ghostball-0` through `5` preserve alternate ball artwork. `paddle-normal-N`, `paddle-magnet-N`, and `paddle-weapon-N` use the original width slot N=6..24; their collision sizes are recorded separately from the padded bitmap dimensions.
- `font.png` is the original 8×8 alphabet and digit sheet, ordered A–Z then 0–9. `font-fame.png` is the alternate high-score alphabet. `digits.png` contains the original ten HUD digits. Transparent pixels preserve palette index zero.
- `laser.png`, `laser-alt.png`, and `paddle-ghost.png` are original weapon and ghost paddle artwork.
- `images/combat.png` reconstructs the static combat scene from raw rectangles in `final.bmp`. `combat-ship.png` is the original 16×33 player sprite. The three 48×47 mouth frames are placed at (240,55). The original player and enemy shots are provided separately.
- `images/combat-meters.png` contains the complete 320×33 original energy panel at scene Y=167. The source stores it as 1,320 contiguous bytes per plane; its decoding requires reshaping these bytes into 320×33 pixels rather than using the assembly copy loop's 96-byte iteration grouping as a bitmap width. The original copper palette changes at Y=166 are applied. Energy fill regions occupy X=27..138 and X=180..291, Y=179..185. The combat scene starts at (0,0), without the normal level's 24-pixel field offset.
- `combat-star.png` is the original 3×3 blue cross, retained in its padded source bitmap. The original combat setup places 61 such sprites and 76 single blue pixels into the background. Star arms and the single-pixel stars use RGB (0,0,187); the cross center uses RGB (0,85,255).
- `assets/audio/*.wav` reconstructs 21 original signed 8-bit Paula playback variants as unsigned 8-bit mono WAV. Some variants share sample bytes but use different source periods. Playback rates derive from the original PAL Paula clock (3,546,895 Hz). Each WAV uses its base period; runtime and offline movie playback apply recovered period changes through the shared `internal/paula` selector.

`assets/manifest.json` records the disk SHA-256, original source filename, byte offsets, bitmap dimensions, visible bounds, palettes, and sound periods. `assets/combat.json` preserves the original boss boundary profile and verified combat constants.

## Level format and verified rules

`level.tab` is 34,560 bytes: 60 grids of 18 columns ×16 rows, with a big-endian 16-bit value per cell. The JSON export retains every value verbatim. The first cell's upper-left gameplay position is (16,24), and each cell occupies 16×8 source pixels.

The low byte indexes the tile directly. Ordinary one-hit bricks occupy IDs 1 through 16. IDs 17 through 48 decrease by 16 per hit. IDs 49 through 64 release an encoded drop and are removed. IDs 65 through 96 decrease by 16 until reaching a drop tile. The completion counter includes nonempty IDs up to 96 and excludes permanent and special obstacles.

For a drop tile, `kind = high_byte >> 2` and `strength = high_byte & 3`. These bits are independent of the visible brick color. Scoring on destruction uses `floor((tile & 15) / 2) << score_multiplier`; the multiplier can be increased by a drop. A reserve life is awarded when the score crosses another 2,048-point threshold. The original starts with four reserves in addition to the active life.

Tile IDs 248 and 249 are paired teleport locations. IDs 250 through 253 temporarily disappear when struck and regenerate after delays derived from `(kind - 249) << 5`. Other special obstacle behavior requires its corresponding rewritten Go rule; the converter itself does not invent behavior.

The original enemy choice table supplies four encoded choices for each normal level. The chosen column depends on the paddle X coordinate modulo four. The low byte is the enemy family. The high-byte flag 0x10 marks lethal paddle contact. Enemy collision sizes are 13×13, 12×25, 16×23, 15×15, 19×22, and 9×18 respectively.

Six combat sequences occur after every ten normal levels. The original player remains at X=16 and moves vertically between Y=0 and 134. The boss starts with 896 health; the player starts with 448. Player shots travel right at 3 pixels per original update. Hits in Y=68..91 deal 8 damage, or 4 in the sixth combat. Enemy shots travel left at 5 pixels per update and deal 64 player damage. Losing combat ends the game immediately. A surviving player returns to the next normal sequence, with the sixth victory ending the campaign.

The default paddle starts at (142,185), with a 36-pixel collision width and a 38×12 visible bitmap. The default ball uses a 5-pixel collision diameter and a 7×7 visible bitmap, with initial velocity (1,-2). Vertical paddle movement is unlocked by bonus 14 rather than being permanently available. The normal width table is preserved in the paddle metadata.

The 8×8 HUD digits use the HUD copper palette at Y=4..11, rather than the gameplay palette used for balls and bricks. This distinction preserves their original cyan appearance. The intro font uses the intro copper palette.

## Scope of fidelity

PNG conversion preserves original bitmap bits, transparency masks and copper color values. WAV conversion preserves original sample values apart from the signed/unsigned representation required by WAV. JSON preserves all level cells and enemy choices. These properties establish asset provenance; they do not alone establish a cycle-exact gameplay port. The rewritten physics, timing, dynamic palette effects, animation schedules, menus, and persistence must be assessed separately against the original.
