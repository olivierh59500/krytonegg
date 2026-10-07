#!/usr/bin/env python3
"""Burn English SRT captions below a recorded native game viewport.

FFmpeg handles H.264 and audio muxing; Pillow draws only video annotations.
The game image is copied without painting over its playfield. Frames stream
through pipes, so the process does not create a PNG sequence or capture the
desktop, microphone, or camera. The original MP4 and SRT remain available.
"""
from __future__ import annotations

import argparse
from dataclasses import dataclass
from fractions import Fraction
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

from PIL import Image, ImageDraw, ImageFont


@dataclass(frozen=True)
class Cue:
    start: float
    end: float
    text: str


def timestamp(value: str) -> float:
    hours, minutes, seconds, milliseconds = map(int, re.split(r"[:,.]", value))
    return hours * 3600 + minutes * 60 + seconds + milliseconds / 1000


def read_subtitles(path: Path) -> list[Cue]:
    """Parse SRT while keeping line breaks and checking chronological order."""
    result = []
    for block in re.split(r"\n\s*\n", path.read_text(encoding="utf-8-sig").strip()):
        lines = block.splitlines()
        timing = next((i for i, line in enumerate(lines) if " --> " in line), None)
        if timing is None:
            continue
        begin, finish = lines[timing].split(" --> ", 1)
        cue = Cue(timestamp(begin.strip()), timestamp(finish.strip()), "\n".join(lines[timing + 1:]))
        if cue.start < 0 or cue.end <= cue.start or (result and cue.start < result[-1].end):
            raise ValueError("SRT cues must be valid and must not overlap")
        result.append(cue)
    if not result:
        raise ValueError("The presentation SRT contains no captions")
    return result


def choose_font(path: str | None, size: int) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    """Use an installed presentation font; game assets are never substituted."""
    if path:
        return ImageFont.truetype(path, size)
    candidates = (
        "/System/Library/Fonts/Supplemental/Arial.ttf",
        "/Library/Fonts/Arial.ttf",
        "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
    )
    for candidate in candidates:
        if Path(candidate).is_file():
            return ImageFont.truetype(candidate, size)
    return ImageFont.load_default(size=size)


def caption_strip(text: str, width: int, height: int, font) -> Image.Image:
    """Wrap a caption in a dedicated footer without covering gameplay pixels."""
    image = Image.new("RGB", (width, height), (8, 12, 18))
    draw = ImageDraw.Draw(image)
    lines: list[str] = []
    for paragraph in text.splitlines():
        current = ""
        for word in paragraph.split():
            proposed = (current + " " + word).strip()
            if current and draw.textlength(proposed, font=font) > width - 80:
                lines.append(current)
                current = word
            else:
                current = proposed
        if current:
            lines.append(current)
    if not lines:
        return image
    line_height = max(1, font.getbbox("Ag")[3] - font.getbbox("Ag")[1] + 10)
    if len(lines) * line_height > height - 12:
        raise ValueError(f"Caption does not fit the footer: {text!r}; increase --footer or reduce --font-size")
    y = (height - len(lines) * line_height) // 2
    for line in lines:
        box = draw.textbbox((0, 0), line, font=font)
        x = (width - draw.textlength(line, font=font)) / 2
        draw.text((x, y - box[1]), line, font=font, fill=(240, 244, 249))
        y += line_height
    return image


def read_frame(stream, length: int) -> bytes | None:
    """Read one whole raw frame even when pipe reads return short chunks."""
    chunks = []
    remaining = length
    while remaining:
        chunk = stream.read(remaining)
        if not chunk:
            if remaining == length:
                return None
            raise RuntimeError("FFmpeg returned a truncated native video frame")
        chunks.append(chunk)
        remaining -= len(chunk)
    return b"".join(chunks)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("video", type=Path)
    parser.add_argument("--srt", type=Path)
    parser.add_argument("-o", "--output", type=Path)
    parser.add_argument("--font")
    parser.add_argument("--font-size", type=int, default=32)
    parser.add_argument("--footer", type=int, default=112)
    args = parser.parse_args()
    video = args.video.resolve()
    srt = (args.srt or video.with_suffix(".srt")).resolve()
    output = (args.output or video.with_name(video.stem + "-captioned.mp4")).resolve()
    if output == video:
        raise ValueError("Choose an output path different from the native recording")
    if args.footer < 24 or args.footer % 2 or args.font_size < 8:
        raise ValueError("The footer must be an even height of at least 24 pixels")
    metadata = json.loads(subprocess.check_output([
        "ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries",
        "stream=width,height,avg_frame_rate", "-of", "json", str(video),
    ]))
    stream = metadata["streams"][0]
    width, height = stream["width"], stream["height"]
    rate = Fraction(stream["avg_frame_rate"])
    if rate <= 0:
        raise ValueError("Native recording has no valid frame rate")
    cues = read_subtitles(srt)
    font = choose_font(args.font, args.font_size)
    strips = [caption_strip(cue.text, width, args.footer, font) for cue in cues]
    blank = caption_strip("", width, args.footer, font)
    canvas = Image.new("RGB", (width, height + args.footer))
    output.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=".presentation-captioned-", suffix=".mp4", dir=output.parent)
    os.close(descriptor)
    temporary_path = Path(temporary)
    # Decoder stderr uses the terminal rather than an unread pipe, which avoids
    # a deadlock on errors. No audio or desktop device is requested.
    decoder = subprocess.Popen([
        "ffmpeg", "-hide_banner", "-loglevel", "error", "-threads", "2", "-i", str(video),
        "-map", "0:v:0", "-an", "-sn", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1",
    ], stdout=subprocess.PIPE)
    encoder = subprocess.Popen([
        "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "rawvideo", "-pix_fmt", "rgb24",
        "-video_size", f"{width}x{height + args.footer}", "-framerate", str(rate), "-i", "pipe:0",
        "-i", str(video), "-i", str(srt), "-map", "0:v:0", "-map", "1:a:0?", "-map", "2:s:0",
        "-c:v", "libx264", "-preset", "veryfast", "-crf", "18", "-pix_fmt", "yuv420p", "-threads", "4",
        "-c:a", "copy", "-c:s", "mov_text", "-metadata:s:s:0", "language=eng",
        "-metadata:s:s:0", "title=English optional text track", "-disposition:s:0", "0",
        "-movflags", "+faststart", str(temporary_path),
    ], stdin=subprocess.PIPE)
    frame_number, cue_index, previous_strip = 0, 0, None
    try:
        while True:
            pixels = read_frame(decoder.stdout, width * height * 3)
            if pixels is None:
                break
            moment = float(Fraction(frame_number, 1) / rate)
            while cue_index < len(cues) and moment >= cues[cue_index].end:
                cue_index += 1
            strip = strips[cue_index] if cue_index < len(cues) and cues[cue_index].start <= moment < cues[cue_index].end else blank
            if strip is not previous_strip:
                canvas.paste(strip, (0, height))
                previous_strip = strip
            canvas.paste(Image.frombytes("RGB", (width, height), pixels), (0, 0))
            encoder.stdin.write(canvas.tobytes())
            frame_number += 1
            if frame_number % max(1, round(float(rate) * 10)) == 0:
                print(f"Captioned {frame_number / float(rate):.0f} seconds of native gameplay", file=sys.stderr, flush=True)
        decoder.stdout.close()
        encoder.stdin.close()
        if decoder.wait() != 0 or encoder.wait() != 0:
            raise RuntimeError("FFmpeg could not finalize the captioned presentation")
        if frame_number == 0:
            raise RuntimeError("The native recording contains no video frames")
        os.replace(temporary_path, output)
    finally:
        for process in (decoder, encoder):
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
        temporary_path.unlink(missing_ok=True)
    print(f"Saved {output} ({frame_number} native frames, English captions, original audio)")


if __name__ == "__main__":
    main()
