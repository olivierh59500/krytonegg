package recording

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeFramesAudioAndEnglishSubtitleMux(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg is not installed")
	}
	output := filepath.Join(t.TempDir(), "native.mp4")
	recorder, err := New(output, 96, 64, 25, 50)
	if err != nil {
		t.Fatal(err)
	}
	if err := recorder.AddCue(0, 300*time.Millisecond, "Native frame capture with offline original audio."); err != nil {
		t.Fatal(err)
	}
	pixels := make([]byte, 96*64*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+3] = 127, 255
	}
	for update := 0; update < 20; update++ {
		if err := recorder.Advance(nil, "intro", false); err != nil {
			t.Fatal(err)
		}
		// Rendering less frequently must preserve the media duration by repeating
		// the latest real image for all presentation frames currently due.
		if update%4 == 3 && recorder.Due() {
			if err := recorder.WriteFrame(pixels); err != nil {
				t.Fatal(err)
			}
		}
	}
	if recorder.frames != 10 || recorder.audioFrames != 17640 || recorder.Duration() != 400*time.Millisecond {
		t.Fatal(recorder.frames, recorder.audioFrames, recorder.Duration())
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Close(); err != nil {
		t.Fatal("closing twice failed", err)
	}
	wave, err := os.ReadFile(strings.TrimSuffix(output, ".mp4") + ".wav")
	if err != nil || len(wave) != 44+17640*4 || binary.LittleEndian.Uint32(wave[40:]) != 17640*4 {
		t.Fatal("incorrect offline WAV length", err)
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("FFprobe is not installed")
	}
	metadata, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_name,nb_frames:stream_tags=language", "-of", "json", output).Output()
	if err != nil || !strings.Contains(string(metadata), `"h264"`) || !strings.Contains(string(metadata), `"aac"`) || !strings.Contains(string(metadata), `"mov_text"`) || !strings.Contains(string(metadata), `"eng"`) {
		t.Fatal(string(metadata), err)
	}
}

func TestSubtitleOverlapsAreRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.srt")
	err := writeSRT(path, []Cue{{0, time.Second, "First"}, {500 * time.Millisecond, 2 * time.Second, "Second"}}, 3*time.Second)
	if err == nil {
		t.Fatal("overlapping English captions were accepted")
	}
}
