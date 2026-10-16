# Preparing the original game data

Krypton Egg's graphics, levels, music and effects are reconstructed locally
from the user's Amiga disk. Neither the ADF nor generated assets are distributed
in this repository.

## Supported disk

Use the [Planet Emulation Amiga ADF catalogue, letter K](https://www.planetemu.net/roms/commodore-amiga-games-adf?page=K)
and select:

```text
Krypton Egg (1990)(HitSoft)[cr QTX]
```

The uncompressed ADF is 901,120 bytes and has SHA-256:

```text
18872e9748bd1b8231da039b8d0b5b493f280e458140f69644f3bbc494902651
```

Extract the `.adf` from its archive and place it at:

```text
previous/Krypton Egg (1990)(HitSoft)[cr QTX].adf
```

## Generate and run

```sh
python3 -m pip install -r tools/requirements.txt
make assets
make run
```

If the disk is stored elsewhere:

```sh
KRYTONEGG_ADF="/path/to/Krypton Egg.adf" make assets
```

The Python pipeline validates the Amiga OFS chains and disk fingerprint before
creating the art, fonts, 60 level layouts, enemy data, tracker modules and
effects used by the Go game. Python is not required after generation.

Keep original disks, generated assets and intermediate files local. Use
`.git/info/exclude`; this repository intentionally has no `.gitignore`.
