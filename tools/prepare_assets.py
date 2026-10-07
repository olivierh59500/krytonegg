#!/usr/bin/env python3
"""Reconstruct ignored game assets from the user's local Amiga disk when needed."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys


ROOT = Path(__file__).resolve().parent.parent
DEFAULT_DISK = ROOT / "previous" / "Krypton Egg (1990)(HitSoft)[cr QTX].adf"
REQUIRED = ("levels.json", "combat.json", "manifest.json", "presentation.json",
            "original/level.tab", "original/intro", "original/halloffame",
            "original/menu.art", "original/fame.art", "original/zz_3.bmp", "original/final.bmp",
            "original/copyrigh.txt", "images/intro.png", "images/background-0.png",
            "images/combat.png", "sprites/font.png", "sprites/digits.png",
            "audio/ceiling.wav", "audio/combat-fire.wav")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--force", action="store_true", help="repeat the lossless reconstruction")
    parser.add_argument("--disk", default=os.environ.get("KRYTONEGG_ADF", str(DEFAULT_DISK)),
                        help="local ADF path; defaults to KRYTONEGG_ADF or previous/")
    args = parser.parse_args()
    disk = Path(args.disk).expanduser().resolve()
    data = ROOT / "assets"
    stamp = ROOT / ".cache" / "assets-recipe.sha256"
    recipe = hashlib.sha256()
    for name in ("extract_adf.py", "presentation_recovery.py"):
        recipe.update((ROOT / "tools" / name).read_bytes())
    signature = recipe.hexdigest()
    complete = all((data / name).is_file() for name in REQUIRED)
    unchanged = stamp.is_file() and stamp.read_text().strip() == signature
    if complete and unchanged and disk.is_file():
        manifest = json.loads((data / "manifest.json").read_text())
        unchanged = hashlib.sha256(disk.read_bytes()).hexdigest() == manifest["source_sha256"]
    if args.force or not complete or not unchanged:
        if not disk.is_file():
            raise SystemExit("Provide the original ADF under previous/ or set KRYTONEGG_ADF, "
                             "then run make assets. Disk data is intentionally absent from Git.")
        subprocess.run([sys.executable, str(ROOT / "tools" / "extract_adf.py"), str(disk),
                        "--output", str(data)], cwd=ROOT, check=True)
        missing = [name for name in REQUIRED if not (data / name).is_file()]
        if missing:
            raise SystemExit(f"Asset reconstruction is incomplete: {missing}")
        stamp.parent.mkdir(parents=True, exist_ok=True)
        stamp.write_text(signature + "\n")
    # The launcher is generated from the exact original intro bitmap, never
    # committed as a second copy of disk artwork in the Android source tree.
    icon = ROOT / "android/app/src/main/res/drawable-nodpi/ic_launcher.png"
    icon.parent.mkdir(parents=True, exist_ok=True)
    if not icon.is_file() or icon.read_bytes() != (data / "images/intro.png").read_bytes():
        shutil.copyfile(data / "images/intro.png", icon)


if __name__ == "__main__":
    main()
