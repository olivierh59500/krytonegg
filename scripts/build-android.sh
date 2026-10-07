#!/bin/sh
set -eu

usage() {
    echo "Usage: $0 [--skip-bind]" >&2
    echo "Build and validate the ARM64 debug APK without installing it." >&2
    echo "KRYTONEGG_ANDROID_TARGET can select additional Ebitengine Android architectures." >&2
}

skip_bind=false
for option in "$@"; do
    case "$option" in
        --skip-bind) skip_bind=true ;;
        --help|-h) usage; exit 0 ;;
        *) usage; exit 2 ;;
    esac
done

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$script_dir/android-env.sh"
cd "$project_root"
python3 "$project_root/tools/prepare_assets.py"
mkdir -p android/app/libs "$GOPATH" "$GOMODCACHE" "$GOCACHE" "$GRADLE_USER_HOME" "$ANDROID_USER_HOME"

module_ebiten_version=$(go list -m -f '{{.Version}}' github.com/hajimehoshi/ebiten/v2)
if [ "$module_ebiten_version" != "$ebiten_version" ]; then
    echo "Ebitengine mismatch: go.mod=$module_ebiten_version, ebitenmobile=$ebiten_version." >&2
    exit 1
fi
if [ "$skip_bind" = false ]; then
    echo "Building the Go/Ebitengine Android library ($android_target)."
    # Build the host binding tools without Cgo so Android-only linker flags do
    # not reach the macOS/Linux linker. Gomobile enables Cgo for the Android JNI
    # library itself. Set both page sizes so RELRO also ends on a 16 KiB boundary.
    CGO_ENABLED=0 \
    CGO_LDFLAGS="${CGO_LDFLAGS:-} -Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384" \
    go run "github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@$ebiten_version" \
        bind \
        -target "$android_target" \
        -androidapi "$android_api" \
        -javapkg "$application_id" \
        -o android/app/libs/krytonegg.aar \
        ./mobile
elif [ ! -f android/app/libs/krytonegg.aar ]; then
    echo "--skip-bind requires a previously generated krytonegg.aar." >&2
    exit 1
fi

# Gradle generates only its default home-directory debug key automatically.
# Create our project-local disposable key explicitly on the first build.
debug_keystore="$android_cache/debug.keystore"
if [ ! -f "$debug_keystore" ]; then
    "$JAVA_HOME/bin/keytool" -genkeypair \
        -keystore "$debug_keystore" \
        -storetype PKCS12 \
        -storepass android \
        -alias androiddebugkey \
        -keypass android \
        -dname "CN=Android Debug,O=Android,C=US" \
        -keyalg RSA \
        -keysize 2048 \
        -validity 10000
fi

echo "Building the debug APK."
"$project_root/android/gradlew" -p "$project_root/android" --console=plain :app:assembleDebug
if [ ! -f "$apk_path" ]; then
    echo "APK missing after build: $apk_path" >&2
    exit 1
fi

echo "Checking package, API levels, signature and 16 KiB alignment."
apk_badging=$("$aapt_path" dump badging "$apk_path")
for expected_badging in \
    "package: name='$application_id'" \
    "sdkVersion:'$android_api'" \
    "targetSdkVersion:'$compile_sdk'" \
    "launchable-activity: name='$application_id.MainActivity'"; do
    case "$apk_badging" in
        *"$expected_badging"*) ;;
        *) echo "Unexpected APK metadata: missing $expected_badging" >&2; exit 1 ;;
    esac
done
"$apksigner_path" verify "$apk_path"
"$zipalign_path" -c -P 16 4 "$apk_path"

# Archive alignment and the native ELF segment alignment are separate checks.
# Both must support Android devices that use 16 KiB memory pages.
readelf_path=
for readelf_candidate in "$android_ndk"/toolchains/llvm/prebuilt/*/bin/llvm-readelf; do
    if [ -x "$readelf_candidate" ]; then
        readelf_path=$readelf_candidate
        break
    fi
done
if [ -z "$readelf_path" ] || ! command -v unzip >/dev/null 2>&1; then
    echo "Native alignment verification requires NDK llvm-readelf and unzip." >&2
    exit 1
fi
verification_dir="$android_cache/verify"
mkdir -p "$verification_dir"
native_libraries=$(unzip -Z -1 "$apk_path" | awk '/^lib\/[^\/]+\/[^\/]+\.so$/ { print }')
if [ -z "$native_libraries" ]; then
    echo "The APK contains no native Go library." >&2
    exit 1
fi
for native_library in $native_libraries; do
    unzip -p "$apk_path" "$native_library" > "$verification_dir/library.so"
    "$readelf_path" -lW "$verification_dir/library.so" | awk '
        function hex(value, result, index_value, digit) {
            value = tolower(value)
            sub(/^0x/, "", value)
            result = 0
            for (index_value = 1; index_value <= length(value); index_value++) {
                digit = index("0123456789abcdef", substr(value, index_value, 1)) - 1
                if (digit < 0) return 0
                result = result * 16 + digit
            }
            return result
        }
        $1 == "LOAD" {
            segments++
            if (hex($NF) < 16384) {
                print "Native LOAD segment has insufficient alignment: " $NF > "/dev/stderr"
                failed = 1
            }
        }
        $1 == "GNU_RELRO" && (hex($3) + hex($6)) % 16384 != 0 {
            print "Native RELRO segment ends outside a 16 KiB page boundary." > "/dev/stderr"
            failed = 1
        }
        END { exit failed || segments == 0 }
    '
done
rm -f "$verification_dir/library.so"
mkdir -p "$project_root/bin"
cp "$apk_path" "$project_root/bin/krytonegg-android.apk"
printf 'APK ready: %s\n' "$apk_path"
