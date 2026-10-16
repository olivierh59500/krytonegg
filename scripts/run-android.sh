#!/bin/sh
set -eu

usage() {
    echo "Usage: $0 [--build-only] [--skip-bind]" >&2
    echo "Build the debug APK, then install and launch it on one authorized device." >&2
    echo "Use ANDROID_SERIAL to select a device when more than one is connected." >&2
}

build_only=false
skip_bind=false
for option in "$@"; do
    case "$option" in
        --build-only) build_only=true ;;
        --skip-bind) skip_bind=true ;;
        --help|-h) usage; exit 0 ;;
        *) usage; exit 2 ;;
    esac
done

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
if [ "$skip_bind" = true ]; then
    "$script_dir/build-android.sh" --skip-bind
else
    "$script_dir/build-android.sh"
fi
if [ "$build_only" = true ]; then
    exit 0
fi
. "$script_dir/android-env.sh"
if [ ! -x "$adb_path" ]; then
    echo "ADB is required; install Android SDK platform-tools." >&2
    exit 1
fi

if [ -z "${ANDROID_SERIAL:-}" ]; then
    device_count=$("$adb_path" devices | awk 'NR > 1 && $2 == "device" { count++ } END { print count + 0 }')
    if [ "$device_count" -ne 1 ]; then
        echo "Exactly one authorized Android device is required; found $device_count. Set ANDROID_SERIAL to select one." >&2
        "$adb_path" devices -l >&2
        exit 1
    fi
    ANDROID_SERIAL=$("$adb_path" devices | awk 'NR > 1 && $2 == "device" { print $1; exit }')
    export ANDROID_SERIAL
fi
if [ "$("$adb_path" get-state)" != "device" ]; then
    echo "The selected Android device is not authorized or connected." >&2
    exit 1
fi

echo "Installing Krypton Egg Go."
"$adb_path" install -r "$apk_path"
echo "Launching Krypton Egg Go."
"$adb_path" shell am start -S -W -n "$application_id/.MainActivity"
