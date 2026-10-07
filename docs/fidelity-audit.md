# Fidelity audit of the supplied Amiga release

The reference is the locally supplied HitSoft Amiga ADF, not a later PC or
mobile release. The original executable is used only for offline inspection.
The native application executes rewritten Go code. Original disks, binaries,
converted assets, emulator states, and reverse-engineering cache files remain
local and are excluded from Git.

## Reference tools and evidence

FS-UAE 3.1.66 was run as an Amiga 500 with the user's Kickstart 1.3 revision
34.5 ROM. Boot, the spacecraft/title image, the original instructions, and
the temple-framed credits were captured directly from that session.
The first original brick round was also reached with recorded emulator mouse
input. Its stable opening frame displays the original zero-based LEVEL 00
banner, score 000000, four reserve ships, and the initial high score 005000.
Screenshots and emulator logs live under `.cache/reverse/` and are deliberately
not repository assets. The credits name the original publisher, programmer,
graphics authors, musician, and sound designer.

Reference sessions run silently. The emulation-core setting is
`uae_sound_output = interrupts`, which preserves Paula interrupt timing
without playing sound. Core sound attenuation is 100, the FS-UAE master volume
is zero, and floppy-drive sounds are disabled. Plain `sound_output = none`
does not override the FS-UAE emulation core; its generated `debug.uae` must be
checked. Automatic input grabbing is disabled.

Ghidra 12.1.4 imported the extracted code hunk as `68000:BE:32:default` with
a project in a temporary directory and its settings inside the local cache.
`tools/reverse/VerifyOriginal.java` disassembles and decompiles the original
combat update, Paula-event queue, and enemy update entries. The local
`ghidra-evidence.c` is reference evidence only. Decompiler warnings and
assembly operands must be reviewed; decompiled output is not production code
and does not establish cycle-exact equivalence.

`tools/reverse/reference_session.py` reproduces the scoped, silent emulator
configuration and extracts the reference code hunk. Its window is hidden by
default; `--visible` enables deliberate inspection.
`tools/reverse/compare_frames.py` reproduces the static first-round RGB
comparison from the local reference captures. `tools/extract_adf.py`
reproduces the bitmap, sample, level, and presentation-data conversions.

## Concrete corrections recovered during this audit

| Behavior | Original evidence | Required native behavior |
| --- | --- | --- |
| Additional ball | Code offsets `0x2f5a..0x2fbe` | Insert a new five-pixel ball at paddle left + 15, paddle top - 4, with velocity `(1,-2)`; no trigonometric rotation of an existing ball. |
| Brick rebound | `0x4bdc..0x4cbc`, `0x4edc..0x4ee2` | Negate the selected horizontal or vertical velocity word. Native circle-contact subdivision may choose a contact differently, but must not introduce a fractional rebound vector. |
| Enemy/ball contact | `0x4a80..0x4ade` | Begin the enemy's destruction, reverse ball vertical velocity, and set horizontal magnitude to one or three with its existing sign, except super-ball or attached-ball cases. |
| Paddle rebound | `0x480e..0x4876` | Reverse vertical velocity; side contact also reverses horizontal velocity. Paddle movement adds arithmetic half of horizontal movement and clamps to three pixels per update. |
| Enemy spawn | `0x24a2..0x24b6` | Spawn at `(146,14)`, with `VY=1` and `VX=1` for even paddle-left coordinates or `-1` for odd coordinates. |
| Enemy animation | `0x265c..0x2666`, `0x2726..0x299e` | Share a nine-update animation counter. Draw offsets belong to scratch render coordinates rather than actual collision positions. Encoded reverse animation uses a ping-pong sequence. |
| Enemy turns | `0x268a..0x26ba` | Share the turn counter across active entities: initial value 99, later reset 169; decrement once per active enemy, rather than once per enemy's independent age. |
| Enemy destruction | `0x27ba..0x282a` | Start destruction at frame 20 and retain the entity in the eight-slot capacity until its death frames finish. Draw the original burst at the unchanged enemy coordinates. |
| Paddle destruction | `0x3512..0x350c`, `0x5820..0x589a` | Decrease the current paddle width slot once per update, then draw slots five through zero using the original spark bank; the default width takes fourteen updates before the reserve decrement and reset. |
| Level acknowledgement | `0x15ac..0x16a0` | Show the zero-based LEVEL banner before creating the paddle; acknowledge it before the separate attached-ball launch. |
| Door cadence | `0x2624..0x2656`, `0x2386..0x24fe` | Wait from counter one to 76; animate opening and closing at two updates per source frame. Capacity eight suspends the next waiting period. |
| Combat ship | `0x5e48` | Start the ship at Y=75, with its recovered X=16 and height 33. |
| Combat mouth | `0x60ba..0x617c` | Wait according to remaining boss energy, then open and close using a six-update mouth step and an energy-dependent open hold; the sixth combat holds it longer. |
| Combat shots | `0x63ae..0x6428`, `0x6458..0x6476` | Boss shot vertical motion occurs on alternate updates. Aim uses signed integer division by 23 and a repeating adjustment of zero, +1, zero, -1. |
| Background selection | `0x2128..0x214e` | Select the next pointer in the original 83-entry cycle on a field rebuild; the pointer is not reset by the start-game routine. |
| Darkness | `0x303e..0x305c` | Reduce each nonzero gameplay color with `(RGB12 >> 1) & 0x777` and restore its original palette after the recovered duration. |

Original velocities are signed 16-bit integers. Floating-point positions in
the high-resolution rewrite can preserve smooth rendering while retaining
these integral velocity rules. In particular, a near-horizontal trajectory
introduced by radial reflection or rotated multiball is a porting error, not
a documented feature of the supplied game.

## Presentation and sound provenance

The original menu cycles 120 source RGB12 colors across its Copper raster
bands. Its selection highlight modifies the eight associated colors. The
intro uses a folded reveal, instructions, and credits before the Play/Scores
menu. The temple screen has the original ten named scores and name entry.
The six combat victories select progressive clue text from the original
copyright data and display it with a rotating Copper palette. The sixth
message in the supplied cracked release refers to a suppressed final scene;
no alternative final-scene branch was found in its observed completion path.
A new ending must not be described as an original recovered sequence.

The extractor also retains the original LEVEL label and its ten 16×17 digits,
which are different artwork from the HUD's 8×8 score glyphs.

The extractor now exports 21 WAV playback variants from original sample
bytes. Additional recovered events are the ceiling and wall bounce, paddle
edge, magnetic release, cannon fire, and combat player fire. Several variants
share source sample bytes but use different Paula periods. Normal paddle
pitch uses `0x29e + (((diameter >> 1) - 2) << 5)`. Ceiling pitch uses
`0x267 + (((diameter >> 1) - 2) << 6)`; side-wall pitch uses `0x32f` as its
base. Ordinary brick pitch uses `0x226 + ((VHPOSR & 31) << 5)`. Special and
metal contacts use lower raster bits and their own periods. Beam-timing
randomness can be expressed with a reproducible native seed, but is not a
claim of identical Amiga raster timing.

## Limits of the comparison

Direct FS-UAE images validate the title artwork, documented controls, credits,
and the first round's arrangement and palette. The native 320×200 ready frame
was compared with the captured original after registering the desktop crop
at `(158,110)` with dimensions `711×443`. The original LEVEL banner and the
remake's additional ready text were excluded from the compared field. The
visible top grid region `(16,56)..(304,79)` agrees in every sampled RGB pixel;
the larger unoccluded field region agrees in 93.68% of sampled pixels. This
uses a window capture with fractional emulator scaling, rather than raw
framebuffer readback: subpixel crop/row selection and different UI state limit
the whole-field percentage. It confirms the source art, first arrangement,
color values, and native grid placement; it is not a general pixel-perfect
claim. Disk-derived tables validate all sixty arrangements,
the six combat boundaries, native counters, and original asset provenance.
The independent native pilot must complete the ordinary game update path
with bounded motion and reaction timing, without changing ball positions,
brick health, score, reserves, drops, or campaign state. Its report is a
progression check of the native rewrite, not a claim that the same controller
has completed every round of the Amiga executable.

The floating-point collision subdivision remains an explicit adaptation.
Simultaneous contacts, exact beam placement, original input polling, and all
bonus combinations have not been proven frame-for-frame equal to the Amiga.
A successful full native campaign and a presentation video therefore support
functional completeness; they do not prove a cycle-exact or pixel-perfect
replacement.

The native sample queue also has a recovered scheduling rule. The queue holds
at most 59 events, pops the newest queued event first, and chooses one of four
Paula channels in the decrementing sequence 1, 0, 3, 2. Starting another
sample on a channel replaces its current voice. This explains why unrestricted
overlapping effects sound different from the source even with identical
exported sample bytes.

The live and offline audio paths now share the pure Go `internal/paula`
selector. It reads all 21 WAV periods from the extraction manifest, gives
explicit source sample/period events priority, and retimes playback by
`basePeriod / eventPeriod`. Both effect paths use four replacing voices in
the recovered channel order, with channels 0 and 3 on the left and 1 and 2
on the right. Update batches are consumed newest first; a generic visual
brick-hit notification does not duplicate its explicit destruction sample.
Ordinary life bookkeeping also does not replay the paddle-destruction sound.
The live path caches retimed 44.1 kHz stereo PCM. The movie renderer performs
all tracker and sample rendering in memory without opening an audio device.

Targeted tests cover every recovered sample name, period changes, native
stereo routing, replacement of a fifth effect voice, paused sample positions,
nonzero original combat-shot audio, and output headroom without clipping.
These tests create no live audio context. Volume headroom is an output
adaptation; Copper interrupt spacing, Paula analogue filtering, and exact
tracker/effect channel preemption are not claimed to be cycle-exact.

## Original manual cheat controls

These rules were recovered from the supplied executable, independently of
WHDLoad patch notes. Before displaying the title, offsets `0x0238..0x0256`
require the Amiga Help key (raw code `0x5f`) and the left mouse button together
and set the authorization word at `0x8820` to `0xffff`. The word begins at zero.
The later controls below require that authorization word.

| Original key | Original action | Evidence |
| --- | --- | --- |
| F10 (`0x59`) | Permanently set the normal-round unlimited-reserve bit at `0x8610`. | `0x1850..0x1862` |
| Escape (`0x45`) | Run the ordinary next-round path, including an intervening combat when due. | `0x186a..0x187e` |
| Control (`0x63`) | Set the round counter to 59 and advance into the sixth combat. | `0x194c..0x1968` |
| F8 (`0x57`), during combat | Set boss energy to eight; one ordinary shot or two sixth-combat shots are then required. | `0x5fce..0x5fe0` |

Unlimited reserves skip the normal death decrement at `0x5820..0x5832`. They
do not prevent combat defeat when player energy reaches zero. No disable
toggle or new-game reset of these cheat flags was found. The startup and
Escape/F10 clues describe working original controls, rather than unused
flavor text. The sixth victory's reference to unlimited lives is nevertheless
unconditional in the supplied cracked release. The native no-cheat campaign
proof and presentation must keep all such assistance disabled. Displaying
the recovered clue text alone does not establish that its activation controls
have also been implemented in the rewrite.
