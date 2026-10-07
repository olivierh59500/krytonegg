#!/usr/bin/env python3
"""Extract the supplied OFS disk and convert its original planar artwork.

Pillow is the only extraction-time dependency. The game needs neither Python nor
an Amiga emulator: it consumes the resulting PNG, JSON, MOD and WAV files.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import struct
import wave

from presentation_recovery import recover_presentation

from PIL import Image

DISK_NAME = "Krypton Egg (1990)(HitSoft)[cr QTX].adf"
GRAPHICS_BASE = 0x11252
PAULA_PAL_CLOCK = 3546895


def word(data: bytes, offset: int) -> int:
    return struct.unpack_from(">H", data, offset)[0]


def longword(data: bytes, offset: int) -> int:
    return struct.unpack_from(">I", data, offset)[0]


def extract_ofs(disk: bytes) -> dict[str, bytes]:
    """Read OFS header/data chains and reject truncated or cyclic chains."""
    if disk[:4] != b"DOS\x00" or len(disk) != 901120:
        raise ValueError("Expected a standard 880 KiB AmigaDOS OFS disk")
    files: dict[str, bytes] = {}
    for sector in range(len(disk) // 512):
        header = disk[sector * 512:(sector + 1) * 512]
        if longword(header, 0) != 2 or longword(header, 508) != 0xFFFFFFFD:
            continue
        name = header[433:433 + header[432]].decode("latin1")
        remaining = longword(header, 324)
        current = longword(header, 16)
        result = bytearray()
        seen: set[int] = set()
        while current and remaining:
            if current in seen or current >= len(disk) // 512:
                raise ValueError(f"Invalid OFS chain for {name}")
            seen.add(current)
            block = disk[current * 512:(current + 1) * 512]
            if longword(block, 0) != 8 or longword(block, 12) > 488:
                raise ValueError(f"Invalid OFS data sector for {name}")
            count = min(longword(block, 12), remaining)
            result.extend(block[24:24 + count])
            remaining -= count
            current = longword(block, 16)
        if remaining:
            raise ValueError(f"Truncated OFS file: {name}")
        files[name] = bytes(result)
    return files


def rgb(value: int) -> tuple[int, int, int]:
    """Expand Amiga 12-bit RGB nibbles without changing their color values."""
    return tuple(((value >> shift) & 15) * 17 for shift in (8, 4, 0))


def copper_rows(code: bytes, offset: int, height: int = 200) -> list[list[int]]:
    """Apply the original copper color-register writes at each raster line."""
    changes: dict[int, list[tuple[int, int]]] = {}
    line = 0
    wrapped = 0
    previous_beam = 0
    for position in range(offset, len(code) - 4, 4):
        register, value = struct.unpack_from(">HH", code, position)
        if (register, value) == (0xFFFF, 0xFFFE):
            break
        if register & 1:
            beam = register >> 8
            if beam < previous_beam:
                wrapped = 256
            previous_beam = beam
            line = max(0, beam + wrapped - 0x3E)
        elif 0x180 <= register <= 0x19E:
            changes.setdefault(line, []).append(((register - 0x180) // 2, value))
    current = [0] * 16
    rows = []
    for y in range(height):
        for index, value in changes.get(y, []):
            current[index] = value
        rows.append(current[:])
    return rows


def planar(data: bytes, width: int, height: int, palette: list[int],
           mask: bytes | None = None, rows: list[list[int]] | None = None,
           transparent_zero: bool = False) -> Image.Image:
    """Decode four contiguous bitplanes; an optional fifth plane is the mask."""
    stride = width // 8
    plane_size = stride * height
    if len(data) < plane_size * 4:
        raise ValueError("Truncated planar bitmap")
    image = Image.new("RGBA", (width, height))
    pixels = image.load()
    for y in range(height):
        colors = [rgb(v) for v in (rows[y] if rows else palette)]
        for x in range(width):
            position, bit = y * stride + x // 8, 7 - x % 8
            index = sum(((data[plane * plane_size + position] >> bit) & 1) << plane
                        for plane in range(4))
            visible = bool(mask[position] >> bit & 1) if mask is not None else True
            if transparent_zero and index == 0:
                visible = False
            pixels[x, y] = (*colors[index], 255 if visible else 0)
    return image


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("disk", nargs="?", default=str(Path("previous") / DISK_NAME))
    parser.add_argument("--output", default="assets")
    args = parser.parse_args()
    disk_path, output = Path(args.disk), Path(args.output)
    disk = disk_path.read_bytes()
    files = extract_ofs(disk)
    # Executables stay in memory and are used only as evidence of bitmap metadata.
    executable = files["kry"]
    code = executable[32:32 + longword(executable, 28) * 4]
    graphics = files["zz_3.bmp"]
    for directory in ("original", "images", "sprites", "audio"):
        (output / directory).mkdir(parents=True, exist_ok=True)
    for name in ("zz_3.bmp", "intro.bmp", "menu.art", "fame.art", "final.bmp",
                 "level.tab", "intro", "halloffame", "copyrigh.txt"):
        (output / "original" / name).write_bytes(files[name])
    palette = [word(code, 0x788C + index * 4) for index in range(16)]
    hud_rows = copper_rows(code, 0x77EA)
    manifest: dict = {
        "source": str(disk_path), "source_sha256": hashlib.sha256(disk).hexdigest(),
        "game_palette_rgb12": palette, "sprites": {}, "backgrounds": [],
        "enemy_types": [], "sounds": {},
    }

    def save_sprite(name: str, image: Image.Image, address: int,
                    source: str = "zz_3.bmp", **metadata) -> None:
        image.save(output / "sprites" / f"{name}.png")
        manifest["sprites"][name] = {
            "source": source, "offset": address - GRAPHICS_BASE,
            "width": image.width, "height": image.height,
            "opaque_bounds": list(image.getbbox() or (0, 0, 0, 0)), **metadata,
        }

    def masked(address: int) -> Image.Image:
        offset = address - GRAPHICS_BASE
        width = (word(graphics, offset) + 2) * 16
        height = word(graphics, offset + 2) + 1
        plane_size = width // 8 * height
        return planar(graphics[offset + 4 + plane_size:], width, height, palette,
                      mask=graphics[offset + 4:offset + 4 + plane_size])

    def packed(address: int, colors: list[int] | None = None) -> Image.Image:
        offset = address - GRAPHICS_BASE
        width, height = (word(graphics, offset) + 1) * 16, word(graphics, offset + 2) + 1
        return planar(graphics[offset + 4:], width, height, colors or palette)

    # This bank contains growing balls, sparks, paddles and paddle destruction.
    address = GRAPHICS_BASE
    for index in range(55):
        image = masked(address)
        save_sprite(f"sprite-{index:02d}", image, address)
        address += 4 + image.width // 8 * image.height * 5
    for index, diameter in enumerate((5, 6, 8, 10, 13, 16)):
        for name, table in (("superball", 0x84E8), ("ghostball", 0x852C)):
            address = longword(code, table + diameter * 4)
            save_sprite(f"{name}-{index}", masked(address), address,
                        collision_diameter=diameter)
    for name, table in (("paddle-normal", 0xB418), ("paddle-magnet", 0xB47C),
                         ("paddle-weapon", 0xB4E0)):
        for size in range(6, 25):
            address = longword(code, table + size * 4)
            save_sprite(f"{name}-{size}", masked(address), address,
                        collision_width=word(code, 0xB3E6 + size * 2))
    for index in range(256):
        address = 0x2B204 + index * 64
        image = planar(graphics[address - GRAPHICS_BASE:], 16, 8, palette)
        save_sprite(f"brick-{index}", image, address, kind=index)

    # Six enemy families have two or three original animation frames each.
    for kind in range(6):
        table = 0x99E2 + kind * 32
        base, last_frame = longword(code, table), word(code, table + 4)
        entry = {"kind": kind, "width": word(code, table - 6),
                 "height": word(code, table - 4), "frames": []}
        for frame in range(last_frame + 1):
            address = base if frame == 0 else base + word(code, table + 4 + frame * 2)
            name = f"enemy-{kind}-{frame}"
            save_sprite(name, masked(address), address)
            entry["frames"].append(name)
        manifest["enemy_types"].append(entry)
    for frame in range(21):
        address = longword(code, 0x9A9C + frame * 4)
        save_sprite(f"enemy-death-{frame}", masked(address), address)
    # Drops use a tightly packed mask and four color planes, with no header.
    for kind in range(1, 28):
        address = longword(code, 0xA0D6 + kind * 4)
        offset = address - GRAPHICS_BASE
        image = planar(graphics[offset + 16:], 16, 8, palette,
                       mask=graphics[offset:offset + 16])
        save_sprite(f"bonus-{kind}", image, address, kind=kind)
        flags = code[0xA0AE + kind]
        frame_count = max(1, flags & 0x3F)
        for frame in range(frame_count):
            frame_address = address + frame * 80
            frame_offset = frame_address - GRAPHICS_BASE
            frame_image = planar(graphics[frame_offset + 16:], 16, 8, palette,
                                 mask=graphics[frame_offset:frame_offset + 16])
            save_sprite(f"bonus-{kind}-{frame}", frame_image, frame_address,
                        kind=kind, frame=frame, animation_flags=flags)
    # The fixed-size door animation frames are drawn directly into the field.
    for frame in range(11):
        address = 0x17476 + frame * 128
        image = planar(graphics[address - GRAPHICS_BASE:], 32, 8, palette)
        save_sprite(f"door-{frame}", image, address)
    for name, address in (("laser", 0x22A44), ("laser-alt", 0x22AC0),
                           ("paddle-ghost", 0x208A2)):
        save_sprite(name, masked(address), address)
    # The original displays a zero-based LEVEL banner before creating a paddle.
    save_sprite("level-label", packed(0x2F2C4), 0x2F2C4)
    for digit in range(10):
        address = 0x2F570 + digit * 0x8C
        save_sprite(f"level-digit-{digit}", packed(address), address)

    for source, name, copper in (("intro.bmp", "intro", 0x7772),
                                 ("menu.art", "menu", 0x808A),
                                 ("fame.art", "fame", 0x82CA)):
        rows = copper_rows(code, copper)
        planar(files[source], 320, 200, rows[0], rows=rows).save(output / "images" / f"{name}.png")
    hud = packed(0x272B0, hud_rows[0])
    # The top bar and the lower eight pixels use different original palettes.
    hud = planar(graphics[0x272B0 - GRAPHICS_BASE + 4:], 320, 24, hud_rows[0], rows=hud_rows)
    left, right = packed(0x281B4), packed(0x28738)
    hud.save(output / "images" / "hud.png")
    left.save(output / "images" / "wall-left.png")
    right.save(output / "images" / "wall-right.png")
    for index in range(83):
        address = longword(code, 0x1EDE + index * 4)
        tile = packed(address)
        screen = Image.new("RGBA", (320, 200))
        for y in range(24, 200, tile.height):
            for x in range(16, 304, tile.width):
                screen.paste(tile, (x, y))
        screen.paste(hud, (0, 0))
        screen.paste(left, (0, 24))
        screen.paste(right, (304, 24))
        screen.save(output / "images" / f"background-{index}.png")
        manifest["backgrounds"].append({"index": index, "address": address,
                                         "tile_width": tile.width, "tile_height": tile.height})

    for source, offset, count, name in (("intro.bmp", 32000, 36, "font"),
                                        ("fame.art", 32000, 40, "font-fame"),
                                        ("zz_3.bmp", 0x2F184 - GRAPHICS_BASE, 10, "digits")):
        sheet = Image.new("RGBA", (count * 8, 8))
        font_palette = copper_rows(code, 0x7772)[0] if source == "intro.bmp" else palette
        digit_rows = hud_rows[4:12] if name == "digits" else None
        for index in range(count):
            sheet.paste(planar(files[source][offset + index * 32:], 8, 8, font_palette,
                               rows=digit_rows, transparent_zero=True), (index * 8, 0))
        sheet.save(output / "sprites" / f"{name}.png")
    manifest["fonts"] = {"font": "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
                           "digits": "0123456789", "glyph_width": 8, "glyph_height": 8}

    levels = files["level.tab"]
    level_data = {"source": f"{disk_path}:level.tab", "width": 18, "height": 16,
                  "encoding": "big-endian uint16, preserved verbatim",
                  "levels": [list(struct.unpack_from(">288H", levels, n * 576)) for n in range(60)],
                  "enemy_choices": [list(struct.unpack_from(">4H", code, 0x973C + n * 8)) for n in range(60)]}
    (output / "levels.json").write_text(json.dumps(level_data, indent=2) + "\n")

    # Combat artwork is stored as four raw, contiguous planes in final.bmp.
    final = files["final.bmp"]
    combat_rows = copper_rows(code, 0x7956)
    combat_palette = combat_rows[0]
    combat = Image.new("RGBA", (320, 200), (*rgb(0), 255))
    for address, width, height, x, y in ((0x11C2A, 160, 17, 160, 0),
                                        (0x1217A, 96, 127, 224, 17),
                                        (0x1394A, 160, 23, 160, 144),
                                        (0x1407A, 320, 33, 0, 167)):
        part = planar(final[address - GRAPHICS_BASE:], width, height, combat_palette,
                      rows=combat_rows[y:y + height])
        combat.paste(part, (x, y))
    combat.save(output / "images" / "combat.png")
    combat.crop((0, 167, 320, 200)).save(output / "images" / "combat-meters.png")
    ship = planar(final[0x11B22 - GRAPHICS_BASE:], 16, 33, combat_palette,
                  transparent_zero=True)
    save_sprite("combat-ship", ship, 0x11B22, source="final.bmp")
    combat.crop((240, 55, 288, 102)).save(output / "sprites" / "combat-mouth-0.png")
    for frame, offset in ((1, 0), (2, 0x468)):
        planar(final[offset:], 48, 47, combat_palette).save(
            output / "sprites" / f"combat-mouth-{frame}.png")
    for name, address in (("combat-shot", 0x15662), ("combat-enemy-shot", 0x1555A)):
        offset = address - GRAPHICS_BASE
        width, height = (word(final, offset) + 2) * 16, word(final, offset + 2) + 1
        plane_size = width // 8 * height
        bitmap = planar(final[offset + 4 + plane_size:], width, height, combat_palette,
                        mask=final[offset + 4:offset + 4 + plane_size])
        save_sprite(name, bitmap, address, source="final.bmp")
    offset = 0x1551A - GRAPHICS_BASE
    star_size = (word(final, offset) + 2) * 2 * (word(final, offset + 2) + 1)
    star = planar(final[offset + 4 + star_size:], (word(final, offset) + 2) * 16,
                  word(final, offset + 2) + 1, combat_palette,
                  mask=final[offset + 4:offset + 4 + star_size])
    save_sprite("combat-star", star, 0x1551A, source="final.bmp")
    combat_data = {"boss_boundary_by_4px_row": list(struct.unpack_from(">40H", code, 0xB37A)),
                   "player_x": 16, "player_height": 33, "mouth_x": 240, "mouth_y": 55,
                   "weakspot_y_min": 68, "weakspot_y_max_exclusive": 92,
                   "boss_health": 896, "player_health": 448}
    (output / "combat.json").write_text(json.dumps(combat_data, indent=2) + "\n")

    # Each playback command gives the original sample address, word count and period.
    sounds = {
        "brick": (0x37C8C, 0x1F4, 0x226), "bonus-brick": (0x38074, 0x7D0, 0x311),
        "paddle": (0x43776, 0x4B0, 0x29E), "enemy-hit": (0x3C43A, 0xEA6, 0x28A),
        "collect": (0x39014, 0x60E, 0x2CB), "extra-life": (0x417D2, 0x8FC, 0x226),
        "game-over": (0x35204, 0x1544, 0x2CB), "start": (0x3270C, 0x157C, 0x2CB),
        "metal": (0x429CA, 0x2EE, 0x230), "special": (0x42FA6, 0x3E8, 0x276),
        "grow": (0x40C1A, 0x5DC, 0x226), "brick-alt": (0x2FAE8, 0x28A, 0x271),
        "teleport": (0x3A914, 0x4E2, 0x5DC), "enemy-fire": (0x3B2D8, 0x44C, 0x31B),
        "combat-end": (0x3E186, 0xEA6, 0x316),
        "ceiling": (0x3BB70, 0x226, 0x267), "wall": (0x3BB70, 0x226, 0x32F),
        "paddle-edge": (0x3BFBC, 0x23F, 0x2CB),
        "magnet-release": (0x39C30, 0x672, 0x2CB),
        "cannon-fire": (0x46304, 0x524, 0x2F8),
        "combat-fire": (0x46304, 0x524, 0x244),
    }
    for name, (address, length_words, period) in sounds.items():
        sample = graphics[address - GRAPHICS_BASE:address - GRAPHICS_BASE + length_words * 2]
        if len(sample) != length_words * 2:
            raise ValueError(f"Truncated audio sample {name}")
        rate = round(PAULA_PAL_CLOCK / period)
        # WAV 8-bit PCM is unsigned; Paula's original samples are signed.
        with wave.open(str(output / "audio" / f"{name}.wav"), "wb") as audio:
            audio.setnchannels(1)
            audio.setsampwidth(1)
            audio.setframerate(rate)
            audio.writeframes(bytes(value ^ 0x80 for value in sample))
        manifest["sounds"][name] = {"source": "zz_3.bmp", "offset": address - GRAPHICS_BASE,
                                     "length_bytes": len(sample), "period": period, "sample_rate": rate}
    (output / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    recover_presentation(code, output)
    print(f"Extracted {len(files)} OFS files; converted original art, 60 levels and {len(sounds)} sounds.")


if __name__ == "__main__":
    main()
