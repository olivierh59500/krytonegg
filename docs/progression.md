# Expert-player progression checks

`internal/player` provides a deterministic simulated expert for the actual Go
game. Its only output is `game.Input`: mouse coordinates, ball-release clicks,
and held fire. It never changes a ball, brick, bonus, score, life counter, round,
or boss energy. A normal campaign starts at the title and must earn every bonus
and clear every stage through the same `World.Tick` path as a person.

The canonical campaign profile delays observations by four PAL updates (80 ms),
limits paddle motion to twelve native pixels per update (600 pixels/second), and
limits acceleration to two pixels per update squared. Initial releases include
a short preparation delay. The player predicts wall, brick, portal, and enemy
contacts from a copied older observation, prepares movement-based paddle
rebounds, aims for visible openings in permanent walls, collects useful drops,
avoids recovered negative bonus identities and lethal creatures, and fires
ordinary weapons. Vertical movement remains locked until original bonus 14 is
caught. Combat steering dodges observed projectiles while aiming at the original
vulnerable band.

The player observes exact native positions, velocities, collision rectangles,
and decoded bonus identities. These are generous perception assumptions for a
very good player; this is a controlled gameplay validator, not a study of human
visual recognition. Prediction is fallible, particularly around simultaneous
contacts and moving enemies. No successful practice selection counts as an
uninterrupted campaign victory.

## Reproducible commands

The headless validator does not initialize Ebitengine, a display, or audio:

```sh
GOCACHE="$PWD/.cache/go-build" GOPROXY=off \
  go run ./cmd/validate-progression -mode campaign -seed 42 \
  -reaction 4 -speed 12 -acceleration 2 \
  -out captures/progression/campaign.json

GOCACHE="$PWD/.cache/go-build" GOPROXY=off \
  go run ./cmd/validate-progression -mode rounds -first 1 -last 60 -seed 42 \
  -reaction 4 -speed 12 -ticks 180000 \
  -out captures/progression/rounds.json

GOCACHE="$PWD/.cache/go-build" GOPROXY=off \
  go run ./cmd/validate-progression -mode combats -seed 42 -reaction 4 -speed 12 \
  -out captures/progression/combats.json
```

`campaign` stops at the first game over, stage time limit, or ten minutes without
reducing remaining brick health or boss energy. `rounds` and `combats` are explicitly labelled practice checks: each
starts the selected authentic stage with the normal initial reserves. They
diagnose all layouts and encounters even if a full campaign run fails. The
report records update counts, simulated seconds, lives, score, remaining bricks,
and outcomes. A failed stage includes its final ball, paddle, active effects,
enemy positions, and surviving bricks. Exit status 1 means progression was not
completed; exit status 2 indicates invalid arguments or asset/report errors.

Add `-trace /tmp/krytonegg-inputs.jsonl` to record every submitted input and its
resulting update, state, paddle, balls, events, lives, and remaining-brick count.
The seed, reaction delay, and speed settings are included in the final JSON.
The original local asset extraction is required for campaign validation.

## Validation status

The canonical [campaign report](../captures/progression/campaign.json) completes
all 60 rounds and six intervening combats in one uninterrupted run, beginning
with five lives and finishing with 21. Seed 42, 80 ms reaction, 600 native
pixels/second, and acceleration two produce 11,479.96 simulated seconds of stage
play, excluding the between-stage pauses. A repeat run produced identical
records for all 66 stages. A second complete campaign with seed 1990,
80 ms reaction and 550 px/s finishes with 17 lives. All six encounters also pass
the separate [combat audit](../captures/progression/combats.json).

Fresh-stage practice is deliberately reported separately. With seed 42, the
80 ms / 600 px/s profile clears 59 of 60 independent starts; a 100 ms / 500 px/s
profile clears 56 of 60. Their [practice coverage](../captures/progression/practice-coverage.json)
covers every original round, retains the failed attempts in both source
reports, and does not present these independent starts as a campaign victory.
The default constructor remains the conservative 100 ms / 350 px/s profile;
the presentation and canonical command explicitly select the verified profile.

These runs exposed and helped correct concrete original-rule mismatches:
fractional corner rebounds, rotated additional balls, enemy contact responses,
shared enemy clocks and destruction frames, cannon penetration through steel,
combat mouth and projectile timing, delayed ship death, and reserves earned
from score thresholds. Cannon penetration was necessary for round 40's five
complete steel rows; its original lower corner bricks supply the enhanced
cannon that opens the barrier. No validation shortcut was needed.

The checks establish functional progression of the rewritten Go rules. They
do not establish a cycle-exact Amiga emulator replacement. Collision positions
and subdivision remain a documented adaptation, and the simulated player has
more precise perception than a person. The source comparison and its limits
are recorded in [fidelity-audit.md](fidelity-audit.md). Local source checksums
are saved with the reports in `captures/progression/source-sha256.txt`.

Meaningful package tests check world immutability, deterministic input/replay,
speed and acceleration limits, the delay and ownership of observations, the
recovered bonus-number preferences, and successful clearance of a small
unchanged-rules fixture. They do not provide bonuses, reserve ships, destructive
shortcuts, or artificial damage during a run.
