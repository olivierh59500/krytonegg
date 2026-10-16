#!/bin/sh
# Rebuild local assets and record only the native game surface, with live sound
# muted and the original soundtrack rendered offline. No desktop capture occurs.
set -eu

usage() {
    printf 'Usage: %s [--prepare-only] [output.mp4]\n' "$0"
    printf 'Set KRYTONEGG_ADF to your local original disk when previous/ is absent.\n'
}

prepare_only=false
if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
    usage
    exit 0
fi
if [ "${1:-}" = "--prepare-only" ]; then
    prepare_only=true
    shift
fi
if [ "$#" -gt 1 ]; then
    usage >&2
    exit 2
fi

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
for executable in go python3 ffmpeg ffprobe; do
    if ! command -v "$executable" >/dev/null 2>&1; then
        printf 'Required tool is missing: %s\n' "$executable" >&2
        exit 1
    fi
done
# Resolve caller-relative inputs before changing the working directory.
if [ -n "${KRYTONEGG_ADF:-}" ]; then
    KRYTONEGG_ADF=$(python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).expanduser().resolve())' "$KRYTONEGG_ADF")
    export KRYTONEGG_ADF
fi
movie_path=${1:-"$project_root/captures/presentation/krypton-egg.mp4"}
movie_path=$(python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).expanduser().resolve())' "$movie_path")
case "$movie_path" in
    *.mp4) ;;
    *) printf 'The presentation output must end in .mp4\n' >&2; exit 2 ;;
esac
movie_stem=${movie_path%.mp4}
captioned_path="$movie_stem-english.mp4"
recording_log="$movie_stem.recording.log"

python3 -c 'from PIL import Image, ImageDraw, ImageFont' >/dev/null

cd "$project_root"
export GOCACHE="$project_root/.cache/go-build"
export CGO_ENABLED=0
mkdir -p "$GOCACHE" "$project_root/bin" "$(dirname -- "$movie_path")"
python3 "$project_root/tools/prepare_assets.py"
go build -trimpath -o "$project_root/bin/krytonegg" .
if [ "$prepare_only" = true ]; then
    printf 'Local assets and native recording binary are ready; no display was opened.\n'
    exit 0
fi

# Each movie starts with a pristine local score table. Repeated exports cannot
# silently replace the original Hall-of-Fame defaults with a previous demo run.
recording_data=$(mktemp -d "$(dirname -- "$movie_path")/.recording-data.XXXXXX")
printf 'Recording four minutes at 50 updates/s; live playback remains muted.\n'
printf 'Native output: %s\nRecording log: %s\n' "$movie_path" "$recording_log"
if "$project_root/bin/krytonegg" -showcase -movie "$movie_path" \
    -movie-ticks 12000 -seed 42 -player-speed 12 -player-reaction 4 \
    -mute -data-dir "$recording_data" \
    >"$recording_log" 2>&1; then
    :
else
    cat "$recording_log" >&2
    exit 1
fi

verify_movie() {
    python3 - "$1" "$2" <<'PY'
import json
from pathlib import Path
import subprocess
import sys

path, expected_height = Path(sys.argv[1]), int(sys.argv[2])
metadata = json.loads(subprocess.check_output([
    "ffprobe", "-v", "error", "-show_entries",
    "stream=codec_type,codec_name,width,height,avg_frame_rate,nb_frames,duration:stream_tags=language:format=duration",
    "-of", "json", str(path),
]))
video = next(stream for stream in metadata["streams"] if stream["codec_type"] == "video")
audio = next(stream for stream in metadata["streams"] if stream["codec_type"] == "audio")
subtitles = [stream for stream in metadata["streams"] if stream["codec_type"] == "subtitle"]
duration = float(metadata["format"]["duration"])
checks = (
    duration >= 240 and abs(duration - 240) < 0.05,
    video["codec_name"] == "h264" and video["width"] == 1280 and video["height"] == expected_height,
    video["avg_frame_rate"] == "50/1" and int(video["nb_frames"]) == 12000,
    audio["codec_name"] == "aac" and abs(float(audio["duration"]) - 240) < 0.05,
    any(stream["codec_name"] == "mov_text" and stream.get("tags", {}).get("language") == "eng" for stream in subtitles),
)
if not all(checks):
    raise SystemExit("Presentation verification failed: " + json.dumps(metadata))
print(f"Verified {path}: {duration:.3f} s, 12000 native frames, H.264/AAC, English subtitles")
PY
}

verify_movie "$movie_path" 800
python3 "$project_root/tools/encode_presentation.py" "$movie_path" --output "$captioned_path"
verify_movie "$captioned_path" 912
printf 'English presentation: %s\nOriginal recording, WAV, SRT, and local data were retained.\n' "$captioned_path"
