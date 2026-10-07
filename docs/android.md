# Android

The Android application runs the same Go simulation, Ebitengine renderer, and
go-zikmu music player as the desktop port. Original assets are embedded in the
Go library. The Java activity only hosts Ebitengine's view and connects Android
lifecycle, navigation, and private storage to the Go application.

## Build

The default APK supports **ARM64 devices running Android 6.0/API 23 or newer**.
Building requires Go 1.26 or newer, JDK 17 or 21, Android SDK Platform 36,
Android Build Tools 36.0.0, Android NDK 28.2.13676358, and `unzip`. The NDK can
be overridden with `ANDROID_NDK_HOME` or `KRYTONEGG_ANDROID_NDK_VERSION`.
Gradle 8.11.1 and Android Gradle Plugin 8.10.1 are pinned by the checked-in
Android project.

```sh
./scripts/build-android.sh
```

The resulting debug APK is:

```text
android/app/build/outputs/apk/debug/app-debug.apk
```

The build also copies it to `bin/krytonegg-android.apk` for installation and
sharing.

The script builds the Go library with the version of `ebitenmobile` pinned to
the project's Ebitengine dependency, then runs the Gradle wrapper. It checks
the APK's package, launch activity, API levels, development signature, archive
alignment, and native-library alignment for 16 KiB pages. A previous AAR can be
reused with `--skip-bind` while changing only the Android host.

```sh
./scripts/build-android.sh --skip-bind
```

SDK and Java locations can be supplied explicitly:

```sh
ANDROID_HOME=/path/to/android-sdk JAVA_HOME=/path/to/jdk \
    ./scripts/build-android.sh
```

On the development Mac, the complete SDK is located at
`/opt/homebrew/share/android-commandlinetools`. The separate Android Studio SDK
under `~/Library/Android/sdk` currently lacks the stable API 36 platform and NDK
used by this build.

For an x86-64 emulator, include both 64-bit architectures:

```sh
KRYTONEGG_ANDROID_TARGET=android/arm64,android/amd64 \
    ./scripts/build-android.sh
```

## Install and launch

Enable USB debugging and authorize the computer on the device, then run:

```sh
./scripts/run-android.sh
```

The script builds, installs, and launches `com.olivierh.krytonegg`. It requires
one authorized device unless `ANDROID_SERIAL` selects a specific device.
`--build-only` builds the APK without installing it, and `--skip-bind` reuses an
existing Go library.

```sh
ANDROID_SERIAL=device-serial ./scripts/run-android.sh --skip-bind
```

The APK uses Gradle's development signing key. A distribution release needs a
separate release-signing configuration.

## Touch controls

The playing field retains its original 320 × 200 coordinates. An additional
panel on the right supplies **PAUSE**, **FIRE**, **SOUND**, and **MENU** controls.
The combined view adapts to the device's landscape resolution while preserving
the original artwork and proportions.

Touch the playing field with one finger and drag to move the paddle relative to
its current position, or move the ship vertically during combat. The initial
contact does not move it.
Vertical paddle movement still requires the original flying-paddle bonus. The
finger remains responsible for steering until released, including when it
crosses the control panel.

Use a second finger on **FIRE** to release an attached ball. Hold it inside the
button to fire an active weapon or shoot during an alien combat. A control
finger cannot take over steering; steering and firing work simultaneously.

Tap **PAUSE** to pause or resume, **SOUND** to toggle audio, and **MENU** to
return to the title. Android's Back control pauses or navigates back, and closes
the activity from the title. The title provides touch buttons for playing,
scores, starting-round selection, help, and the construction set.

In the construction set, drag across the grid to paint at the finger's actual
position. The controls select tiles, bonuses and strength, switch erasing,
save or reload the custom level, test it, and return to the previous screen.
Sliding a painting finger over a control does not press that control.

## Lifecycle and local data

The host uses landscape orientation and immersive fullscreen. Android's
`onPause` suspends the Ebitengine view, which stops simulation updates and
suspends the audio backend. `onResume` restores the view and audio. Interrupted
rounds remain paused until the player taps **RESUME**. Go game construction
happens on the first update after the view has initialized its Android context.

High scores and custom levels are saved in the application's private files
directory, supplied by Android's `getFilesDir()`. No external-storage
permission or network access is required. Android removes this local data
when the app is uninstalled or its storage is cleared.

All game rules, rendering, touch handling, tracker replay, and save-file logic
remain in Go. Android packaging uses Ebitengine's standard generated JNI
library and Android audio/graphics integration.

## Verification

The touch controller has 13 platform-independent tests for initial-contact
anchoring, vertical deltas, reordered fingers, simultaneous movement and fire,
button press edges, gesture ownership, release and reacquisition, reused pointer
IDs, menu changes, and construction-set painting:

```sh
go test ./internal/touch ./internal/presentation
```

Seven mobile presentation tests also exercise paused reversed-controls
gestures, overlay navigation, simultaneous menu presses, the editor's touch
paint/save/reload/test flow, and sidebar hit mapping at a higher resolution
with fractional scaling.

The APK was installed and exercised on a **Google Pixel 10a**, running API 37,
with a 1080 × 2424 display and 4 KiB memory pages. Sustained ADB touch gestures
and captures verified:

- Scores and Back navigation, starting a round, and relative field dragging.
  A 100-unit drag moved the paddle from 160 to 260 without releasing the
  attached ball; the FIRE control then launched it.
- Pause stability, with identical full-frame and playing-field captures taken
  minutes apart while paused. Returning after Home required an explicit RESUME, and
  Android Back navigated from a paused game to the title.
- Editor painting and saving. The recovered private `custom.level` contained
  exactly 576 bytes, and comparison against the original level table showed
  only the five painted cells changed. The custom-level test rendered those
  bricks, and Back returned from the test to the editor and then to the title.
- An alien-combat check completed 45 native Go updates and emitted the expected
  `krytonegg_android_check` report. The capture showed the original boss,
  projectiles, meters, and controls. The Android audio stream initialized
  successfully with `AAUDIO_OK`.

Simultaneous two-finger movement and fire are covered by the controller tests;
the physical-device automation used single-pointer ADB gestures. The build
checks validate 16 KiB archive and native-library alignment separately from
the Pixel's 4 KiB runtime. No emulator system image or AVD was used.

The build's native-library checks follow Android's
[16 KiB page-size guidance](https://developer.android.com/guide/practices/page-sizes).
