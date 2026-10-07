#!/usr/bin/env python3
"""Compare a native ready frame with the original emulator's desktop capture.

This measures static first-round artwork, not replay or physics equivalence.
The original level banner and the remake's additional ready text are excluded.
Use a raw native framebuffer if available; fractional desktop scaling reduces
whole-field agreement even when the artwork and placement are identical.
"""
from __future__ import annotations
import argparse
import json
from pathlib import Path
from PIL import Image


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("original", type=Path)
    parser.add_argument("native", type=Path)
    parser.add_argument("--crop", type=int, nargs=4, default=[158, 110, 711, 443], metavar=("X", "Y", "WIDTH", "HEIGHT"))
    parser.add_argument("--output", type=Path, default=Path(".cache/reverse/comparison.json"))
    args = parser.parse_args()
    original = Image.open(args.original).convert("RGB")
    native = Image.open(args.native).convert("RGB")
    if native.size != (320, 200):
        raise ValueError("The native comparison frame must be the 320 by 200 logical framebuffer")
    x, y, width, height = args.crop
    sampled = original.crop((x, y, x + width, y + height)).resize(native.size, Image.Resampling.NEAREST)

    def fraction(bounds: tuple[int, int, int, int], exclude_banner: bool = False) -> float:
        left, top, right, bottom = bounds
        equal = count = 0
        for row in range(top, bottom):
            for column in range(left, right):
                if exclude_banner and 80 <= column < 224 and 79 <= row < 120:
                    continue
                equal += sampled.getpixel((column, row)) == native.getpixel((column, row))
                count += 1
        return equal / count

    result = {
        "original": str(args.original), "native": str(args.native),
        "original_crop": args.crop,
        "field_mask": {"rectangle": [16, 24, 304, 152], "exclude": [80, 79, 224, 120]},
        "field_exact_rgb_fraction": fraction((16, 24, 304, 152), True),
        "top_grid_rectangle": [16, 56, 304, 79],
        "top_grid_exact_rgb_fraction": fraction((16, 56, 304, 79)),
        "left_wing_exact_rgb_fraction": fraction((16, 56, 80, 112)),
        "right_wing_exact_rgb_fraction": fraction((224, 56, 304, 112)),
        "limitations": "Static desktop capture with fractional emulator scaling and differing Ready/banner UI states; not frame-by-frame equivalence.",
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, indent=2) + "\n")
    sampled.save(args.output.with_name("original-level-00-logical.png"))
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()
