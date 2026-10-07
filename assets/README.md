# Local game data

This directory contains Go embedding code and locally reconstructed game data.
The reference disk, raw files, PNG/WAV conversions, level tables, palettes, and
all other disk-derived assets are excluded from Git and its history.

Supply the original ADF under the ignored `previous/` directory, then run:

```sh
python3 -m pip install -r tools/requirements.txt
make assets
```

The extraction and recovery scripts remain versioned. They reconstruct the
original data without downloading another release or generating replacement art.
The compiled game embeds the locally generated data; it does not require an
emulator or execute the Amiga program.
