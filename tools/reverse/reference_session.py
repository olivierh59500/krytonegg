#!/usr/bin/env python3
"""Prepare an isolated, silent FS-UAE reference session and optional Ghidra data.

The extracted executable is evidence for reverse engineering only. It is never
embedded in, linked with, or executed by the native Go remake.
"""
from __future__ import annotations
import argparse
from pathlib import Path
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from extract_adf import DISK_NAME, extract_ofs, longword


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--rom", type=Path, required=True)
    parser.add_argument("--disk", type=Path, default=Path("previous") / DISK_NAME)
    parser.add_argument("--emulator", type=Path, default=Path("/Applications/FS-UAE.app/Contents/MacOS/fs-uae"))
    parser.add_argument("--run", action="store_true")
    parser.add_argument("--visible", action="store_true", help="Show the emulator window for deliberate visual inspection")
    args = parser.parse_args()
    root = Path.cwd()
    cache = root / ".cache" / "reverse"
    cache.mkdir(parents=True, exist_ok=True)
    executable = extract_ofs(args.disk.read_bytes())["kry"]
    code = executable[32:32 + longword(executable, 28) * 4]
    (cache / "kry-code.bin").write_bytes(code)
    base = cache / "fs-uae"
    (base / "screenshots").mkdir(parents=True, exist_ok=True)
    configuration = {
        "amiga_model": "A500",
        "kickstart_file": str(args.rom.resolve()),
        "floppy_drive_0": str(args.disk.resolve()),
        "base_dir": str(base),
        "window_width": "960",
        "window_height": "600",
        "window_hidden": "0" if args.visible else "1",
        "fullscreen": "0",
        "video_sync": "0",
        "initial_input_grab": "0",
        "automatic_input_grab": "0",
        "volume": "0",
        # UAE-prefixed options reach the emulation core. The similarly named
        # unprefixed sound_output option does not disable Paula in FS-UAE 3.1.
        "uae_sound_output": "interrupts",
        "uae_sound_volume": "100",
        "uae_sound_volume_paula": "100",
        "uae_sound_volume_cd": "100",
        "floppy_drive_volume": "0",
        "floppy_drive_0_sounds": "0",
        "screenshots_output_dir": str(base / "screenshots"),
        "keyboard_key_f11": "action_screenshot",
    }
    config = cache / "original.fs-uae"
    config.write_text("[fs-uae]\n" + "\n".join(f"{key} = {value}" for key, value in configuration.items()) + "\n")
    print(f"Prepared {config}")
    if args.run:
        process = subprocess.Popen([str(args.emulator), str(config)])
        print(f"Reference emulator PID: {process.pid}", flush=True)
        try:
            raise SystemExit(process.wait())
        except KeyboardInterrupt:
            process.terminate()
            process.wait()


if __name__ == "__main__":
    main()
