#!/bin/sh
# Shared Android settings. Source this file from scripts/build-android.sh or run-android.sh.

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
android_cache="$project_root/.cache/android"
android_sdk=${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}
java_home_path=${JAVA_HOME:-}
ebiten_version=v2.10.2
android_api=23
compile_sdk=36
build_tools_version=36.0.0
ndk_version=${KRYTONEGG_ANDROID_NDK_VERSION:-28.2.13676358}
application_id=com.olivierh.krytonegg
android_target=${KRYTONEGG_ANDROID_TARGET:-android/arm64}

if [ -z "$android_sdk" ]; then
    for sdk_candidate in /opt/homebrew/share/android-commandlinetools "$HOME/Library/Android/sdk" "$HOME/Android/Sdk"; do
        if [ -f "$sdk_candidate/platforms/android-$compile_sdk/android.jar" ]; then
            android_sdk=$sdk_candidate
            break
        fi
    done
fi
if [ -z "$java_home_path" ]; then
    for java_candidate in /opt/homebrew/opt/openjdk@21 /opt/homebrew/opt/openjdk@17 "/Applications/Android Studio.app/Contents/jbr/Contents/Home"; do
        if [ -x "$java_candidate/bin/java" ]; then
            java_home_path=$java_candidate
            break
        fi
    done
fi
if [ -z "$java_home_path" ] && command -v java >/dev/null 2>&1; then
    # Linux installations commonly expose JAVA_HOME through readlink.
    java_executable=$(command -v java)
    if command -v readlink >/dev/null 2>&1; then
        java_resolved=$(readlink -f "$java_executable" 2>/dev/null || true)
        if [ -n "$java_resolved" ]; then
            java_home_path=$(dirname -- "$(dirname -- "$java_resolved")")
        fi
    fi
fi

if [ -z "$android_sdk" ] || [ ! -f "$android_sdk/platforms/android-$compile_sdk/android.jar" ]; then
    echo "Android SDK $compile_sdk is required; set ANDROID_HOME or ANDROID_SDK_ROOT." >&2
    exit 1
fi
for android_tool in aapt apksigner zipalign; do
    if [ ! -x "$android_sdk/build-tools/$build_tools_version/$android_tool" ]; then
        echo "Android Build Tools $build_tools_version are incomplete ($android_tool)." >&2
        exit 1
    fi
done
android_ndk=${ANDROID_NDK_HOME:-"$android_sdk/ndk/$ndk_version"}
if [ ! -f "$android_ndk/meta/platforms.json" ]; then
    echo "Android NDK $ndk_version is required; set ANDROID_NDK_HOME to a compatible installation." >&2
    exit 1
fi
if [ -z "$java_home_path" ] || [ ! -x "$java_home_path/bin/java" ]; then
    echo "Java 17 or 21 is required; set JAVA_HOME." >&2
    exit 1
fi
if ! command -v go >/dev/null 2>&1; then
    echo "Go 1.26 or newer is required in PATH." >&2
    exit 1
fi
if [ ! -x "$project_root/android/gradlew" ]; then
    echo "The Android Gradle wrapper is missing." >&2
    exit 1
fi

export ANDROID_HOME="$android_sdk"
export ANDROID_SDK_ROOT="$android_sdk"
export ANDROID_NDK_HOME="$android_ndk"
export ANDROID_USER_HOME="$android_cache/sdk-user"
export JAVA_HOME="$java_home_path"
export PATH="$java_home_path/bin:$android_sdk/platform-tools:$PATH"
export GOWORK=off
export GOTOOLCHAIN=${GOTOOLCHAIN:-local}
# Build tools may create caches and a development signing key. All defaults stay
# inside the checkout; explicit KRYTONEGG_ANDROID_* overrides remain available.
export GOPATH=${KRYTONEGG_ANDROID_GOPATH:-"$android_cache/go"}
export GOMODCACHE=${KRYTONEGG_ANDROID_GOMODCACHE:-"$android_cache/go/pkg/mod"}
export GOCACHE=${KRYTONEGG_ANDROID_GOCACHE:-"$android_cache/go-build"}
export GRADLE_USER_HOME=${KRYTONEGG_ANDROID_GRADLE_HOME:-"$android_cache/gradle"}

adb_path="$android_sdk/platform-tools/adb"
apk_path="$project_root/android/app/build/outputs/apk/debug/app-debug.apk"
aapt_path="$android_sdk/build-tools/$build_tools_version/aapt"
apksigner_path="$android_sdk/build-tools/$build_tools_version/apksigner"
zipalign_path="$android_sdk/build-tools/$build_tools_version/zipalign"
