#!/usr/bin/env python3
"""Recover presentation palette data from the supplied game executable.

The output contains color-register data only. No original instructions are
copied into the native application's assets or interpreted at runtime.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import struct


def recover_presentation(code: bytes, output: Path) -> None:
    """Write the original Copper data needed by the rewritten Go renderer."""
    def copper(address: int) -> list[list[int]]:
        result = []
        for offset in range(address, len(code) - 3, 4):
            register, value = struct.unpack_from(">HH", code, offset)
            result.append([register, value])
            if register == 0xFFFF and value == 0xFFFE:
                return result
        raise ValueError("Unterminated original Copper list")

    data = {
        "source": "kry original color-register data",
        "menu_copper_address": 0x808A,
        "menu_copper": copper(0x808A),
        "menu_cycle_address": 0x872E,
        "menu_cycle": list(struct.unpack_from(">120H", code, 0x872E)),
        "menu_highlight_word_offsets": [0x186, 0x1CE, 0x20E],
        "fame_copper": copper(0x82CA),
        "intro_copper": copper(0x7772),
        "interlude_copper": copper(0x7A4E),
        "interlude_font_source": "original/final.bmp",
        "interlude_font_graphics_offset": 0x175C6 - 0x11252,
        "interlude_font_glyphs": "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" + "()?!.':",
    }
    manifest = json.loads((output / "manifest.json").read_text())
    sprite_names = {item["offset"]: name for name, item in manifest["sprites"].items()
                    if name.startswith("sprite-")}
    data["paddle_death_sprites"] = [
        "sprites/" + sprite_names[struct.unpack_from(">I", code, 0xB418 + slot * 4)[0] - 0x11252] + ".png"
        for slot in range(6)
    ]
    (output / "presentation.json").write_text(json.dumps(data, indent=2) + "\n")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("disk", nargs="?", default="previous/Krypton Egg (1990)(HitSoft)[cr QTX].adf")
    parser.add_argument("--output", default="assets")
    args = parser.parse_args()
    # Import the existing checked OFS reader instead of duplicating filesystem
    # traversal. This module can also be called by extract_adf.main directly.
    from extract_adf import extract_ofs, longword
    executable = extract_ofs(Path(args.disk).read_bytes())["kry"]
    code = executable[32:32 + longword(executable, 28) * 4]
    recover_presentation(code, Path(args.output))


if __name__ == "__main__":
    main()
