# Port status

The game runs as a native Go application using Ebitengine. The simulation,
input, drawing, audio integration, menus, editor, and persistence are rewritten
Go code. Original machine code is not executed at runtime.

## Implemented

- All 60 original brick arrangements load from the supplied disk's unchanged
  18 by 16 big-endian table. Layered bricks retain their original tile IDs,
  embedded bonus identifiers, and two-bit strength values.
- The six original horizontal combats follow rounds 10, 20, 30, 40, 50, and
  60. They use original boss, ship, projectile, background, and meter artwork,
  plus recovered boss boundary, health, movement, damage, and shot constants.
- Original source images, transparent sprites, textures, bitmap fonts,
  15 Paula sound samples, and both tracker songs are embedded. The asset
  converter records their source locations and reproduces the conversions
  from the supplied ADF.
- Normal and magnetic paddles, weapon paddles, ball sizes, super balls,
  ghost balls, bonuses, enemies, the opening door, and destruction effects
  use the original sprite banks. Native collision dimensions are kept
  separate from transparent sprite padding and decorative outlines.
- The original bonus dispatch identifiers implement growth, sensitivity,
  score multiplication, reversed controls, magnetic catches, reserves,
  extra balls, enlargement, darkness, speed changes, autopilot, vertical
  movement, ghost effects, shields, cannons, random selection, shrinkage,
  and four combinable weapon modes. Identifier 5 remains inert because its
  source flag has no identified consumer.
- Enemies use the original per-round four-choice table, paddle-coordinate
  selection, dimensions, animation periods, direction changes, contact
  flags, and capacity limits. The opening-door cadence is derived from the
  original instruction sequence.
- Scoring retains the recovered unsigned 16-bit behavior, score multiplier,
  reserve awards at 2,048-point thresholds, and four reserves plus the active
  ship at the start of a run.
- Play, scores, pause, help, game-over, and campaign-completion paths are
  implemented. Original Space, F1, F2, and F3 actions are supported, with
  additional keyboard, touch, gamepad, fullscreen, and mute controls.
- The construction screen edits original tile codes, attaches bonuses,
  saves and loads a 576-byte level, and tests it. Leaving a test restores the
  campaign rather than replacing it with the test level.
- Desktop presentation defaults to a 1,280 by 800 window with nearest-neighbor,
  integer scaling. Resizing preserves the original 320 by 200 composition
  through centered letterboxing. Simulation positions retain fractional
  precision at higher output resolutions.
- An ARM64 Android APK runs the same Go game and go-zikmu audio integration on
  API 23 or newer. Relative field dragging and independent fire controls share
  a 400 by 200 view with an original-art sidebar. Uniform fractional scaling
  fits landscape screens; title navigation and the editor work with touch.
  Android lifecycle suspension stops the view and audio, and local saves use
  the application's private files directory.
- Fixed seeds permit reproducible simulation. Command-line round and combat
  selection, automatic update runs, and PNG capture support review.

## Adaptations and remaining differences

Recovered constants and unchanged assets establish provenance and substantial
behavioral fidelity. They do not establish a cycle-exact emulator replacement.
The rewritten collision solver subdivides motion, resolves circle/rectangle
contacts, and uses floating-point positions. This deliberate adaptation
prevents fast balls from crossing thin bricks between collision checks.
Original rebound and paddle-motion constants inform the implementation, but
corner contacts, simultaneous hits, and fine timing can differ.

Several presentation and persistence details remain simplified:

- The menu's copper-driven moving colors are represented by a static decoded
  image. The original animated introductory reveal is not reproduced.
- The local score file stores one best score. The original named ten-entry
  hall-of-fame workflow is not reproduced.
- Original post-boss screens that progressively reveal cheat instructions
  are omitted; combat victory advances the campaign directly.
- Combat star placement uses the selected replay seed in place of original
  raster-timing randomness. Background selection uses the current round's
  position in the recovered texture bank rather than reproducing the
  complete original runtime texture-selection state.
- WAV files use recovered base sample periods. Events that changed Paula
  sample periods dynamically in the original do not all vary pitch here.
- The construction screen preserves the original level format and art while
  providing new mouse, keyboard, and touch interactions.

These differences should remain visible in fidelity claims. Original gameplay
video and disk-derived evidence are described in
[original-reference.md](original-reference.md); asset formats and recovered
constants are described in [asset-extraction.md](asset-extraction.md).

## Validation scope

Headless tests cover original level decoding, campaign indexing, deterministic
state, brick layering and regeneration, fast-ball and corner collisions,
paddle response, multiball life handling, bonus dispatch and strength,
enemy contacts, combat transitions and damage, native counters, and tracker
audio decoding. They establish the implemented rules' consistency, rather
than proving frame-for-frame equivalence with the Amiga.

Native smoke runs and captures exercise application startup, graphics, audio
initialization, and automatic updates. The printed smoke report records the
resulting world state; it is not a claim that every campaign round was cleared
or that every original behavior was verified.

The Android APK was installed and tested on a Google Pixel 10a running API 37.
Device checks verified relative movement without unintended launch, firing,
scores and Back navigation, stable pause captures, Home/return behavior, editor
painting and its original-format save, returning from a custom-level test,
and a 45-update alien-combat render. Android audio initialization completed
successfully. Headless touch and presentation tests cover simultaneous fingers,
gesture ownership, reversed-control reanchoring, and editor reload behavior;
physical automation used single-pointer ADB gestures. Package checks validate
the signature, metadata, and 16 KiB archive and native-library alignment.
See [android.md](android.md) for builds, controls, and detailed verification.
