# Original Amiga reference

This port targets the HitSoft Amiga release supplied in
`previous/Krypton Egg (1990)(HitSoft)[cr QTX].adf`. The Atari ST release and the
later DOS, Windows, and mobile remakes are useful historical context, but are
not authoritative sources for this port's graphics or level arrangements.

## Evidence and confidence

The supplied disk is the authoritative source for original art, audio, and
level data. The original game's own title instructions and gameplay were
inspected in an [Amiga longplay](https://www.youtube.com/watch?v=AdAs4n-__pc).
An [original Amiga box scan](https://images.launchbox-app.com/r2_41488d36-b9af-4ea7-8e0d-070a77f3a211.jpg)
confirms 60 brick-breaking sequences, six intervening combats, and a
construction set. These are direct observations of the original product.

The [LaunchBox Amiga image archive](https://gamesdb.launchbox-app.com/games/images/23973-krypton-egg)
provides two clean, native 320 by 200 gameplay captures. The [GamesNostalgia
Amiga archive](https://gamesnostalgia.com/game/krypton-egg) provides additional
captures and a short gameplay clip. These are useful visual references, but
their selected frames do not identify the level number and some emulator
captures have a missing HUD frame. The supplied disk's decoded map ordering
must take precedence over guesses based on screenshot order.

The [original developer interview](https://www.atarilegend.com/interviews/48)
confirms that Alexandre Kral wrote the original 68000 game and that Xavier
later made the PC versions. This supports treating the later releases as
distinct implementations.

## Screen geometry

Coordinates below describe the clean 320 by 200 Amiga captures. They are
observations of rendered pixels rather than recovered collision constants.

| Element | Observed native geometry | Confidence |
| --- | --- | --- |
| Complete logical image | 320 by 200 | Confirmed |
| HUD frame | Top 17 pixels, approximately y = 0 through 16 | Confirmed visually |
| Left decorative wall | x = 0 through 15 below the HUD | Confirmed |
| Right decorative wall | x = 304 through 319 below the HUD | Confirmed |
| Inner playfield | x = 16 through 303; bottom is open | Confirmed |
| Brick cell | 16 by 8; face approximately 14 by 6 with inset edges | Confirmed |
| Initial rectangle rows | y = 56, 64, 72, 80, 88, 96, 104 | Confirmed |
| Normal paddle collision width | 36 pixels; source bitmap is 38 by 12 | Recovered from the disk |
| Normal paddle initial upper-left position | x = 142, y = 185 | Recovered from the disk |
| Initial ball collision diameter | 5 pixels; source bitmap is 7 by 7 | Recovered from the disk |
| Enemy entrance | Centered below the life capsule, at approximately x = 146, y = 14 | Recovered from the disk |

The clean initial-rectangle reference is [this gameplay
image](https://images.launchbox-app.com/258146d1-63a0-4a9a-9f2e-f5ada9a24c07.png).
The [second clean
image](https://images.launchbox-app.com/68b22e84-ace6-493a-8398-d584f4451f5b.png)
shows another pyramid arrangement. Its place in the original sequence is not
identified by the archive.

The HUD consists of three cream-edged rounded capsules: score on the left,
ship/life indicator in the center, and high score on the right. The playfield
uses repeating green or pink geometric patterns and bright multicolored
decorative walls. Brick faces use beveled gray, cyan, green, yellow, orange,
and red shading. Different recordings show different backgrounds for the
same initial brick arrangement. Disk extraction establishes 83 texture
pointers selected from a sequential cycle; the first texture is a green
diamond pattern. The cycle's relationship to prior title/menu activity
requires further runtime comparison.

High-resolution presentation should preserve these relationships, the native
brick proportions, and the source artwork. Exact native collision geometry
must be distinguished from the optional output scaling.

## Controls and initial game flow

The title's own instruction screen explicitly documents these controls:

| Original control | Original action |
| --- | --- |
| Space | Pause the game |
| F1 | Switch PAL/NTSC mode |
| F2 | Release the ball |
| F3 | End the current game |

Mouse-controlled horizontal paddle movement is visible throughout the Amiga
recording. Vertical paddle movement is unlocked by bonus 14 during ordinary
rounds. Shooter combat fixes the ship at x = 16 and permits vertical movement
between y = 0 and 134. The exact relationship of
mouse buttons to ball release and repeated firing was not established from
video alone; F2 is the directly documented release key.

The original menu displays Play and Scores buttons. The title sequence shows
the metallic game logo and a spacecraft over a starfield. The separate high
score screen uses a gold temple frame over green masonry.

The first playable arrangement in the longplay is the gray rectangular core
with colored wings, matching the first supplied disk map. A colored pyramid
follows it. The longplay begins gameplay at approximately 2:54 and its first
HUD ship count is 04. Disk analysis subsequently confirms that this counts
four spare ships in addition to the active ship, giving five initial lives.

## Ordinary rounds

Directly observed mechanics include angled paddle rebounds, bonuses falling
from broken bricks, multiple simultaneous balls, ball enlargement, variable
paddle width, upward weapons, vertical paddle freedom, and a pulsing
protective halo around the paddle. Several effects can coexist. A short
[archived Amiga gameplay clip](https://t.gamesnostalgia.net/movie/k/r/krypton-egg/gameplay.mp4)
shows these effects clearly.

Small animated creatures emerge very early in the initial round, within a
few seconds of the first launch. They travel through the field and interfere
with balls. Their motion varies by creature. The source disk includes six
distinct animated types, including a shark-like figure, small angular flying
figures, and a round glowing figure. Its recovered tables allow eight active
enemies and four encoded choices per round, selected using the paddle's
horizontal coordinate modulo four. The encoded 0x1000 flag denotes lethal
paddle contact. The native turn period is 169 updates and the animation
period is six updates. Spawn timing also includes an opening door animation;
its instruction sequence and cadence have now been recovered from the disk.
This resolves the earlier estimate based solely on observing video frames.

Disk extraction establishes 27 bonus graphics and their original numeric
identifiers. A level cell's low byte identifies the brick graphic, rather
than the falling effect: IDs 49 through 64 are bonus-bearing graphics, IDs
65 through 96 are layered variants, and IDs 97 through 112 are permanent
collidable obstacles. The high byte encodes the bonus identifier in its upper
six bits and its strength in the lower two bits. Reducing a layered brick
subtracts 16 from its graphic identifier while preserving this metadata.

Recovered dispatch behavior establishes the following bonus identifiers:

| ID | Recovered effect |
| --- | --- |
| 1 | Paddle growth |
| 2 | Increased mouse sensitivity |
| 3 | Score multiplier |
| 4 | Reversed controls |
| 5 | Sets a flag with no identified consumer |
| 6 | Magnetic ball catch |
| 7 | Additional reserves |
| 8 | Additional ball |
| 9 | Ball enlargement |
| 10 | Darkness, lasting `64 × (strength + 1)` updates |
| 11, 12 | Acceleration and deceleration |
| 13 | Autopilot, lasting `512 × (strength + 1)` updates |
| 14 | Vertical paddle control |
| 15 | Ghost paddle, lasting `32 × (strength + 1)` updates |
| 16 | Enemy shield, with strength plus one charges |
| 17 | Cannon |
| 18 | Super ball |
| 19 | Ghost ball, with a `256 × (strength + 1)` counter |
| 20 | Fixed ghost paddle |
| 21 | Enhanced cannon |
| 22 | Random effect, excluding IDs 12 and 22 |
| 23 | Paddle shrinkage |
| 24 through 27 | Weapon mode bits 1, 2, 4, and 8 |

These identities are derived from original dispatch instructions, rather than
guesses from the drops' appearance. Runtime stacking, movement, collision,
and animation behavior still require comparison with the original. Full
data formats and provenance are recorded in [asset-extraction.md](asset-extraction.md).

## Shooter combats

The original box confirms six combats among the 60 sequences. In the
longplay, the first combat occurs around 14:53, after the first group of brick
rounds. The player ship becomes vertical and flies at the left of a scrolling
starfield while a large organic green boss occupies the right. The player
fires pale projectiles to the right. Disk analysis confirms a fixed ship
x-coordinate, player energy of 448, and boss energy of 896. The boss sends orange projectiles left
in streams and spreading patterns. Organic barriers occupy the top and
bottom, and separate player and boss energy meters appear at the bottom.
Brick-breaking balls and the ordinary brick grid are absent during this
sequence. Player shots travel three pixels per native update and deal eight
damage within y = 68 through 91; the sixth combat reduces this to four.
Boss shots travel left at five pixels per update and deal 64 player damage.
Combat loss ends the game even when brick-round reserves remain. The
recording does not establish whether the continuous player fire is automatic
or results from a held button.

The first boss defeat is followed by a colored text screen revealing the
first part of the original unlimited-lives cheat. It explicitly promises the
next clue at level 20. The title instructions explain that the complete cheat
is revealed after all monsters have been killed. Defeating the first boss
therefore reveals a clue; it does not itself confirm an immediate automatic
unlimited-lives reward.

For historical comparison, the [WHDLoad install
author's notes](https://www.whdload.de/games/KryptonEgg.html) document a cheat
enabled with the left mouse button at startup, then Escape for skipping a
level and F10 for unlimited lives. These notes describe a patched installation
and should not be mistaken for the normal original gameplay controls.

## Fidelity limits to track

The port can preserve the supplied disk's assets and arrangements exactly
while expressing simulation logic in new Go code. That does not establish
exact reproduction of original timing or behavior. The following details
remain unresolved by the references inspected here:

- The unconsumed flag set by bonus 5 and exact runtime stacking behavior.
- Death animation scheduling and fine simultaneous-contact behavior.
- Original speed ramps, rebound angles, friction, and PAL/NTSC timing.
- Boss projectile schedules and dynamic palette effects in all six combats.
- Original background selection and construction-set interaction.

Any approximations in these areas should be documented as new implementation
choices until tested against the supplied Amiga release.
