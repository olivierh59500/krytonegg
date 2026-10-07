# Original presentation recovery

The native presentation uses disk graphics, fonts, names, messages, and color
register data. Its clocks, input handling, rendering, and JSON persistence are
new Go code. Original instructions are neither embedded nor interpreted.

`tools/presentation_recovery.py` writes the local generated
`assets/presentation.json` during asset conversion. This records original
Copper register pairs and the menu's color table. The JSON and original asset
files remain outside Git; the conversion code is versioned.

The following addresses refer to the `kry` code hunk before relocation. They
are evidence for the rewritten behavior, not runtime entry points.

| Feature | Original evidence | Native behavior |
| --- | --- | --- |
| Intro image reveal | `0xbba..0xc56` | 200 bottom-up steps; each unfinished upper row repeats the current source row. Fire advances to credits. |
| Intro credits | `0xd00..0xe32`, text at `0x8916` | The original copyright text scrolls upwards one native pixel per update. A second fire edge skips to the menu. |
| Menu moving color | `0x3ee..0x422`, Copper at `0x808a`, colors at `0x872e` | The original 120-color cycle moves through the logo's 32 scanlines. |
| Menu highlight | `0x422..0x48a`, offsets at `0x8728` | Original eight-register palette blocks dim and brighten the selected option. |
| Score defaults | `menu.art` bytes `32000..32267` | All ten original names and scores are decoded from the unchanged disk data. |
| Score rows | `0x874..0x92c` | The original fame font is drawn at `(64,68)`, with 24 cells per row and eight-pixel row spacing. Its palette comes from the fame Copper list at `0x82ca`. |
| Name entry | `0x7ac..0x80c` | New entries allow 16 characters, following the source loop's `0x10` limit. Keyboard and touch entry are native adaptations. The source defaults contain longer names, which remain intact. |
| Menu credits | Copyright data at `0x8826` | The ten original encoded copyright rows use the fame screen and font. |
| Pre-paddle level banner | `0x15ac..0x169c` | Original zero-based level label and 16 by 17 digit sprites appear on the source black rectangle before paddle creation. A fire edge acknowledges it before a second fire releases the ball. |
| Paddle destruction | `0x3480..0x3500`, pointer bank at `0xb418` | The width slot decreases once per update through normal paddles, then through six recovered spark sprites. Default width uses fourteen updates before the reserve is consumed. |
| Combat messages | `0x65a0..0x6760`; text pointers at `0xb3c8` | Six original victory messages and the original combat-loss message use the 43 source 8 by 16 monochrome glyphs at `final.bmp` offset `0x6374`, loaded at address `0x175c6`. |
| Combat text color | `0x6734..0x6760`, Copper at `0x7a4e` | The 180 original scanline colors rotate by two positions per update. |

The supplied cracked ADF's round-60 victory path selects its sixth message
unconditionally. That message mentions the unlimited-lives code and an omitted
final scene. Source inspection found no alternate ending scene in this disk's
completion path. The port retains the supplied message rather than inventing
an ending or silently changing its wording.

The score table is stored locally in `halloffame.json`. A previous
`highscore.json` file migrates into a named entry without replacing the disk
defaults. The file is replaced atomically after name confirmation. A malformed
file leaves defaults available and exposes a persistence error to the caller.
The source's `CMP.W` followed by `BGT` ranks signed sixteen-bit values even
though decimal scores are unsigned. This original limitation is retained:
scores from 32,768 to 65,535 do not displace the positive default entries.

The sequential background cursor persists across return-to-menu actions. Each
real level-start event consumes the next original texture; the hidden initial
world does not advance it. Darkness halves each original RGB nibble across
the lower game field, including its actors, while retaining the HUD palette.
Enemy drawing applies source frame offsets, and original death frames retain
the enemy's native coordinates and asymmetric bitmap bounds.

These recovered assets and routines establish the source of each presentation
feature. The new implementation advances at the selected application update
rate; original disk delays, blitter timing, raster polling, and instruction
execution are not reproduced cycle by cycle. Keyboard and touch name entry,
the on-screen alphabet, and local JSON saves adapt the original mouse entry
and disk writing to modern systems.

# Native video recording

`internal/recording` accepts RGBA pixels read directly from Ebitengine's game
surface. It sends those images through a bounded raw-frame pipe to FFmpeg's
H.264 encoder. It does not capture the desktop, other applications, microphone,
camera, or system audio, and does not create a large PNG sequence.

The recording clock consumes one event frame per simulation update. Original
tracker music is rendered offline through `go-zikmu`; original WAV samples are
resampled and mixed in Go. Music transitions reset the selected module, and
pauses retain both tracker and sample positions while writing silence. A
shared sample selector can supply the runtime's sample and playback-rate
decisions. No live speaker context is created by the recorder.

The final output consists of an H.264/AAC MP4, a stereo PCM WAV, and an English
SRT file. The MP4 includes an English `mov_text` subtitle track. If Ebitengine
skips a draw between updates, the latest actual native image is repeated for
the due media frames, keeping video and offline audio clocks aligned. This
does not interpolate or alter the simulation.

For captions that remain visible in players without subtitle support:

```sh
python3 tools/encode_presentation.py captures/presentation/krytonegg.mp4
```

This optional post-processing step streams the recorded video through Pillow
and FFmpeg. It appends a 112-pixel annotation footer below the game viewport
and draws the English captions there. It preserves the original gameplay
pixels, copies the recorded AAC audio, and retains an optional English text
track. A system presentation font is used only for video annotations.

Recording infrastructure tests validate actual H.264/AAC/subtitle muxing,
exact PCM lengths, frame-clock synchronization, pause behavior, tracker reset,
subtitle overlap checks, and malformed WAV rejection. A one-second synthetic
encoder fixture also verifies the caption-footer pipeline. These tests verify
the recording process; they do not assert campaign progression or prove
original-game equivalence. Only real simulation events and recorded gameplay
can establish what a presentation run achieved.

## Reproducing the four-minute export

After the ordinary campaign audit has been reviewed, run:

```sh
export KRYTONEGG_ADF="/absolute/path/to/the/original.adf"
./scripts/record-presentation.sh
```

The script reconstructs current local assets, builds the native binary with
CGO disabled and the project-local build cache, and creates a fresh score-data
directory for the movie. It records exactly 12,000 PAL updates with live sound
muted. The application continues updating while unfocused, so the recording
needs no global input or desktop capture. The default outputs are under the
ignored `captures/presentation/` directory.

The director presents thirty seconds of intro and options, 190 seconds of
ordinary campaign inputs, and a separate twenty-second first-boss practice
selection. That totals four minutes with 210 seconds allocated to gameplay.
The explicit player settings match the final canonical controller: seed 42,
four updates of observation delay (80 ms), twelve native pixels/update maximum
speed (600 pixels/second), and the default two pixels/update squared
acceleration limit. The on-screen caption derives its timing and speed from
the actual options rather than assuming fixed settings.
Practice selection is identified in the English subtitles and does not count
as uninterrupted campaign progression. The final footage was reviewed at the
option transitions, during the continuous campaign segment, and during the
boss and its victory message. No game-over or artificial progress shortcut
replaces the ordinary campaign segment. The final boss-practice victory message
is the recovered source presentation and is retained in the footage.

The script checks H.264, AAC, English subtitle metadata, exact 50 fps, 12,000
video frames, the expected viewport dimensions, and a four-minute duration.
It then creates a second MP4 with the English footer and repeats those checks.
Native MP4, captioned MP4, PCM WAV, SRT, recording log, and isolated local score
data remain available for review. Pass another `.mp4` path to select a different
local destination.

`./scripts/record-presentation.sh --prepare-only` regenerates and compiles the
same source without opening a display or starting a recording. This supports
a fresh source-only checkout with `KRYTONEGG_ADF` pointing to the user's local
input; original disk data need not be copied into the checkout or Git archive.

## Optional original codes

The source release authorizes its optional codes with HELP and the left mouse
button at startup. The native desktop mapping is Insert plus the held left
mouse button during the intro. Authorization persists until the application
exits; it is disabled by default. After authorization in ordinary manual play:

- F10 enables process-persistent unlimited reserves. It does not protect the
  ship during alien combat.
- Escape follows the normal next-round transition, including each tenth-round
  boss, without adding points or reserves.
- Either Control key during a brick round selects the sixth combat while preserving current score
  and reserves.
- During combat, F8 sets the original low boss-energy value of eight; ordinary
  player shots are still required to win.

The application forcibly clears authorization and unlimited reserves for
expert playback, showcase, smoke checks, and every movie recording. These
modes also ignore physical keyboard, mouse, touch, and gamepad controls that
could interfere with their simulated player. The final campaign report records
`cheats_enabled: false`. Intent-level workflow tests exercise authorization,
persistence, automatic-mode exclusion, tenth-round routing, and the requirement
for actual boss shots; they do not fake hardware key events.

Standard gamepad bottom-button press edges acknowledge intro, banner, clue,
and name-entry modals in manual play, matching ordinary launch controls.
