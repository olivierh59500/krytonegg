# Port status

The game is a native Go application using Ebitengine. Simulation, input,
rendering, menus, construction tools, persistence, and audio integration are
rewritten Go code. Original instructions are neither executed nor interpreted
at runtime. The compiled application embeds data reconstructed from the user's
local original ADF.

## Implemented model and presentation

- All 60 unchanged 18 by 16 original layouts retain tile identifiers, layered
  strength, encoded bonuses, permanent and regenerating obstacles, and paired
  teleporters. The six original alien encounters follow each group of ten
  rounds, with recovered ship, mouth, projectile, damage, and energy rules.
- Ball velocities retain native integer rules for brick, paddle, enemy,
  multiball, and speed-bonus responses. The modern collision solver subdivides
  motion using floating-point positions. Original paddle movement contributes
  to rebounds, and vertical steering requires the original flying bonus.
- Recovered bonus dispatch covers size and sensitivity, reversed controls,
  magnet, score multiplication, reserves and extra balls, darkness, speed,
  autopilot, vertical movement, ghost effects, shields, cannons, random
  selection, super balls, and combinable weapons. Identifier 5 remains inert
  because no consumer of its source flag has been identified.
- Enemies use original round selections, shared counters, movement and lethal
  flags, scratch drawing offsets, capacity limits, and death frames. The door
  follows recovered native cadence. Paddle destruction decreases its width
  slot once per update and consumes the reserve after the original animation.
- Scoring wraps as an unsigned 16-bit value. Reserve awards use recovered
  2,048-point thresholds; a run starts with four reserves and the active ship.
  The Hall of Fame deliberately retains the source's signed-word ranking
  limitation while rendering scores as unsigned decimal values.
- The original folded intro reveal, scrolling instructions, animated menu
  colors and selection palettes, copyright screen, and zero-based LEVEL
  banner are implemented. The banner is acknowledged before paddle creation
  and the separate ball release. Backgrounds follow the persistent original
  83-entry cursor. Darkness halves original color nibbles across the game field.
- The Hall of Fame displays ten original named defaults using its source font
  and palette. A qualifying completed or F3-ended run requests a name before
  insertion. Native keyboard and touch entry accept sixteen characters; longer
  disk defaults are retained. Local JSON saves are atomic, and the earlier
  single-score file migrates without discarding the defaults.
- Original victory clues and combat-loss text use the recovered monochrome
  font from the final 688 bytes of `final.bmp` and its rotating palette. The supplied cracked release always selects its
  sixth clue at final victory; no alternate final-scene branch was found in
  that disk's completion path.
- Original graphics, masks, textures, fonts, tracker modules, and 21 recovered
  WAV playback variants are used. Live and offline sound share recovered
  event selection, native period changes, newest-first queue ordering, four
  replacing Paula-style channels, and stereo channel placement. Tracker music
  is decoded and mixed in pure Go by `go-zikmu`.
- Optional original keyboard codes require explicit manual startup authorization
  with Insert and the held left mouse button during the intro. Their reserve,
  round-selection, final-encounter, and boss-energy functions are available only
  in normal manual play. Expert, showcase, smoke, and all movie modes forcibly
  disable them; they are not used by progression validation or presentation.
- The construction set paints original tile codes and bonus strengths, saves
  and reloads the unchanged 576-byte format, and tests a custom level. Returning
  from a test restores the campaign and preserves the editor's work.
- Desktop output defaults to 1,280 by 800 with nearest-neighbor integer scaling
  and centered letterboxing around the original 320 by 200 composition.
  Keyboard, mouse, gamepad, pause, fullscreen, and mute controls are available.
- Android shares the Go game and adds relative field dragging, independent
  fire, navigation, name entry, and construction controls in a 400 by 200
  composition. Saves use private application storage; lifecycle suspension
  pauses an active round until the player resumes it.
- A deterministic simulated expert submits ordinary inputs with delayed
  observations and bounded speed and acceleration. It does not move balls,
  remove bricks, grant bonuses, award reserves, or alter boss energy. Headless
  campaign and labelled practice reports use the same simulation update path.
- Native video capture streams Ebitengine's actual surface into H.264. Original
  audio is rendered offline while live playback remains muted. MP4/AAC output,
  English SRT and text tracks, and optional captions below the game viewport
  are implemented without desktop, microphone, or system-audio capture.

## Comparison and remaining limits

FS-UAE and Ghidra inspection established the source of the recovered assets,
constants, presentation sequences, and concrete corrections. A registered
first-round comparison matched every sampled pixel in the visible top grid;
93.68% of the larger unoccluded field agreed. This comparison used a fractionally
scaled emulator window capture with selected UI areas excluded. It supports
art, palette, arrangement, and placement fidelity; it is not a general claim
of a bit-perfect or cycle-exact replacement.

The floating-point collision subdivision remains a deliberate adaptation.
Corner choices, simultaneous contacts, original raster polling, exact hardware
interrupt timing, and every bonus combination have not been proven equivalent
frame by frame. Combat stars and raster-dependent sound variation use seeded
native randomness instead of the Amiga's current beam position. Paula queue
scheduling is represented per update, and software resampling and output gain
adapt playback to the host audio system.

Keyboard and touch name entry, JSON persistence, modern construction controls,
additional help text, and phone navigation adapt the original interactions.
The simulated expert observes exact game geometry and decoded bonus identities;
these are generous perception assumptions for a controlled gameplay validator.

The final-source ordinary campaign audit completed all sixty rounds and six
encounters through submitted inputs, with original codes disabled throughout.
[validation.json](validation.json) records the canonical seed 42, an 80 ms
observation delay, a maximum 600 native pixels/second, and acceleration limited
to two pixels/update squared. Both repeated runs cleared the same 66 stages
with matching stage records. The first round took 81.12 simulated seconds;
the complete audit totals 11,479.96 simulated seconds and ends with 21 lives.
Each boss encounter ended with 448 player energy. The local full replay is
`captures/progression/campaign.json`; [progression.md](progression.md) explains
its input-only scope and reproduction.

Loading layouts, passing unit tests, winning separately selected practice
fights, and producing a movie remain distinct from that ordinary campaign
check. Neither the successful native audit nor a presentation movie proves
frame-for-frame equivalence with the Amiga executable.

## Validation and reproducibility

Package tests cover source decoding, deterministic state and input ownership,
native counters and rebounds, collisions, brick layering and regeneration,
bonus dispatch, enemy and combat behavior, reserve timing, touch interaction,
named scores, presentation clocks, tracker audio, sample selection and channel
replacement, video clocks, and actual H.264/AAC/subtitle muxing. Native smoke
captures exercise the Ebitengine render. A two-second presentation prototype
contains exactly 100 native 1,280 by 800 frames at 50 fps, synchronized AAC,
and an English text track. Its captioned version retains those frames and adds
only a 112-pixel footer. The final canonical four-minute presentation contains
exactly 12,000 actual Ebitengine frames at 50 fps, synchronized AAC audio, and
an English text subtitle track. Its captioned version is 1,280 by 912 pixels;
the game remains 1,280 by 800 and the subtitles occupy the additional footer.
Visual checks cover the original intro and menu, named scores, readable help,
the construction set, Android controls, continuous campaign footage, the boss,
and its corrected original-font victory message. The movie uses the same seed,
reaction delay, speed and acceleration limits as the canonical controller.
The complete 60-round campaign proof remains separate from the four-minute
presentation, whose last encounter is explicitly labelled as practice.

The earlier Android APK was installed and checked on a Google Pixel 10a running
API 37. That session verified movement, firing, pause, Back, Home/return, editor
save/reload, and a combat render. Those observations predate the latest source
fidelity corrections; a newly built APK must be distinguished from that earlier
device check. [android.md](android.md) records the platform workflow and checks.

The ADF, `previous/`, `.DS_Store`, converted PNG/WAV files, raw disk data, generated
metadata, modules, launcher artwork, and reverse-engineering caches are excluded
from Git. The requested cleanup removed forbidden data from the three existing
historical revisions and pruned 496 original asset objects. The subsequent audit
found no forbidden paths in reachable history. Recovery scripts and Go embedding
code remain versioned; README review screenshots are rendered captures, rather
than extracted source asset files. `tools/scrub_git_history.py --audit` repeats
that path audit without changing the repository.

Set `KRYTONEGG_ADF` to the local disk and run `make assets` to reconstruct the
ignored data. `make build`, `make test`, and the Android build check the current
asset recipe automatically. The recording workflow is documented in
[presentation.md](presentation.md); source evidence and comparison limits are
in [fidelity-audit.md](fidelity-audit.md) and
[asset-extraction.md](asset-extraction.md).
