# Krypton Egg — Go / Ebitengine

A native Go port of the Amiga game Krypton Egg. All game logic,
physics, input, rendering, construction tools, and persistence are rewritten in
Go. The game does not run the Amiga executable or require an emulator.

The artwork, bitmap fonts, 60 level layouts, enemy selections, two tracker
modules, and 15 sound samples come exclusively from that disk. Music is replayed
by **go-zikmu** through Ebitengine's audio backend. Everything needed to play is
embedded in the executable.

## Run

Requires Go 1.26 or newer and a desktop graphics environment.

```sh
go run .
```

The initial window is **1280 × 800**. Resize it or press **F** for fullscreen.
Rendering uses integer nearest-neighbor zoom and centered letterboxing to retain
the original 320 × 200 proportions. The simulation keeps native coordinates;
moving objects are rendered directly at the higher output resolution.

```sh
make build
./bin/krytonegg
```

The dependencies are pinned in `go.mod`. No C compiler, Python, Kickstart ROM, or
Amiga installation is required to play. Python and Pillow are used only if you
choose to reproduce the asset conversion.

## Controls

| Control | Action |
| --- | --- |
| Mouse / arrows / WASD | Move the paddle; vertical movement requires its original bonus |
| Left click / F2 / Enter | Release an attached ball |
| Hold left click / F2 | Fire when a weapon is active; fire in an alien combat |
| Space | Pause or resume during a round or combat |
| F1 | Switch PAL (50 updates/s) and NTSC (60 updates/s) |
| F3 | End the current game and return to the title |
| P / Escape | Additional pause controls |
| F / F11 | Toggle fullscreen |
| M | Toggle sound |
| H | Open or close help |
| Q | Quit from the title or pause screen |

The title retains **PLAY** and **SCORES**. Click the corresponding button, or use
Up/Down and Enter. Left/Right selects a starting round. Press **E** on the title
to open the construction set. Touch input and standard gamepad movement/fire
are also supported.

## Campaign and construction set

The original 60 grids include layered bricks, permanent obstacles, disappearing
bricks, regenerating obstacles, and paired teleporters. The original encoded
bonuses control paddle size, magnet, mouse sensitivity, reversed movement,
score multiplier, darkness, spare lives, additional balls, ball size/speed,
autopilot, vertical movement, ghost paddle, shield, cannons, super/ghost balls,
second paddle, random effects, and four laser modes. The native enemy choice
table supplies the six monster families and lethal-contact flags.

An alien combat follows each group of ten rounds. The six fights use the
original ship, animated mouth, projectiles, collision profile, and energy meters.
Losing an alien combat ends the run, as on this disk.

The construction set edits an 18 × 16 grid with original tiles:

- Left click paints; right click erases.
- Left/Right or the mouse wheel selects a tile.
- **B** selects an encoded bonus; **V** selects its two-bit strength.
- **S** saves; **L** reloads; **Enter** tests the edited level.
- **Escape** returns from the test to the editor, then from the editor to the title.

Custom levels use the original 576-byte big-endian format. High scores and
`custom.level` are stored in the OS user configuration directory under
`krytonegg/`. Use `-data-dir` to select another location.

```sh
go run . -level 12
go run . -combat 1
go run . -scale 5 -fullscreen
go run . -editor -data-dir ./captures/local-data
go run . -custom ./captures/local-data/custom.level
```

## Verification and asset provenance

```sh
make test
make vet
CGO_ENABLED=0 go build -o bin/krytonegg .
./bin/krytonegg -mute -smoke 260 -capture captures/round.png -data-dir .cache/check-data
./bin/krytonegg -mute -combat 1 -smoke 45 -capture captures/combat.png -data-dir .cache/check-data
```

The automatic check uses deterministic input and exits after the requested
number of updates. Captures contain the actual Ebitengine render. Simulation
tests cover native level decoding, collisions, paddle momentum, bonuses, enemy
hazards, weapons, teleporters, regeneration, scoring, and all six combat
transitions. Audio tests render both original modules using go-zikmu.

To reproduce the lossless conversion:

```sh
python3 -m pip install -r tools/requirements.txt
make assets
```

See [asset extraction](docs/asset-extraction.md),
[original references](docs/original-reference.md), and
[port status](docs/port-status.md) for source offsets, recovered rules, and the
remaining limits of behavioral fidelity. Assets are original; the rewritten
simulation uses modern collision resolution rather than reproducing every
68000 instruction or Amiga hardware timing effect. Menu palette animation,
the original multi-entry name-entry scoreboard, and post-boss cheat-message
presentation are not reproduced in full. Bonus ID 5 has no identified consumer
in this disk's behavior table and is retained as inert data.

## Project layout

- `internal/game`: deterministic simulation without renderer/audio dependencies.
- `internal/presentation`: Ebitengine renderer, input, title, construction set, and saves.
- `internal/sound`: original PCM effects and go-zikmu tracker playback.
- `assets`: embedded original data, converted bitmaps/audio, and provenance manifests.
- `tools/extract_adf.py`: reproducible OFS/planar/Paula conversion.
- `previous`: untouched reference disk.

Original game: HitSoft; programming Alexandre Kral; graphics Alexandre and
Xavier Kral; music Jean-Pierre Vidos; sound effects Philippe Deneyer. Original
assets retain their original ownership.
