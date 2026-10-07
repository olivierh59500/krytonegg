package recording

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"krytonegg/internal/game"
)

// Cue stores an English presentation subtitle on the recording's media clock.
type Cue struct {
	Start, End time.Duration
	Text       string
}

// Recorder streams actual RGBA game images to H.264 while writing offline PCM.
// It keeps one audio chunk and one caller-owned video frame in memory.
type Recorder struct {
	path, videoPath, audioPath   string
	width, height, fps, tps      int
	updates, frames, audioFrames int64
	encoder                      *exec.Cmd
	video                        io.WriteCloser
	audio                        *os.File
	mixer                        *Mixer
	stderr                       bytes.Buffer
	cues                         []Cue
	closed                       bool
	closeError                   error
}

// New starts FFmpeg's raw-image encoder without accessing any capture device.
// Width and height must be even; tick rate is the simulation update rate.
func New(path string, width, height, fps, ticks int) (*Recorder, error) {
	if path == "" || !strings.EqualFold(filepath.Ext(path), ".mp4") || width <= 0 || height <= 0 || width%2 != 0 || height%2 != 0 || fps <= 0 || ticks <= 0 {
		return nil, fmt.Errorf("recording requires an MP4 path, even dimensions, and positive frame/update rates")
	}
	encoderPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("recording requires FFmpeg: %w", err)
	}
	mixer, err := NewMixer()
	if err != nil {
		return nil, err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}
	video, err := os.CreateTemp(directory, ".presentation-video-*.mp4")
	if err != nil {
		return nil, err
	}
	videoPath := video.Name()
	if err := video.Close(); err != nil {
		os.Remove(videoPath)
		return nil, err
	}
	audio, err := os.CreateTemp(directory, ".presentation-audio-*.wav")
	if err != nil {
		os.Remove(videoPath)
		return nil, err
	}
	if _, err := audio.Write(make([]byte, 44)); err != nil {
		audio.Close()
		os.Remove(audio.Name())
		os.Remove(videoPath)
		return nil, err
	}
	r := &Recorder{path: path, videoPath: videoPath, audioPath: audio.Name(), width: width, height: height, fps: fps, tps: ticks, audio: audio, mixer: mixer}
	r.encoder = exec.Command(encoderPath, "-hide_banner", "-loglevel", "error", "-y", "-f", "rawvideo", "-pixel_format", "rgba", "-video_size", fmt.Sprintf("%dx%d", width, height), "-framerate", fmt.Sprint(fps), "-i", "pipe:0", "-an", "-c:v", "libx264", "-preset", "veryfast", "-crf", "18", "-pix_fmt", "yuv420p", "-threads", "4", "-movflags", "+faststart", videoPath)
	r.encoder.Stderr = &r.stderr
	r.video, err = r.encoder.StdinPipe()
	if err == nil {
		err = r.encoder.Start()
	}
	if err != nil {
		audio.Close()
		os.Remove(audio.Name())
		os.Remove(videoPath)
		return nil, fmt.Errorf("start video encoder: %w", err)
	}
	return r, nil
}

// SetSampleSelector shares exact runtime sample and playback-rate decisions.
func (r *Recorder) SetSampleSelector(selector func(game.Event) (string, float64)) {
	r.mixer.SetSampleSelector(selector)
}

// Advance records one simulation update's events, music choice, and pause state.
// Live audio may remain muted: this offline clock never opens a speaker.
func (r *Recorder) Advance(events []game.Event, music string, paused bool) error {
	if r.closed {
		return fmt.Errorf("recording is closed")
	}
	r.updates++
	target := r.updates * SampleRate / int64(r.tps)
	pcm, err := r.mixer.Render(int(target-r.audioFrames), events, music, paused)
	if err != nil {
		return err
	}
	if _, err := r.audio.Write(pcm); err != nil {
		return err
	}
	r.audioFrames = target
	return nil
}

// Due reports whether a new native frame should be read from Ebitengine.
func (r *Recorder) Due() bool { return !r.closed && r.frames < r.updates*int64(r.fps)/int64(r.tps) }

// WriteFrame encodes the supplied native render for every due media frame.
// Repeating the latest actual image bridges skipped draws without losing audio
// synchronization. It does not invent world state or simulate between frames.
func (r *Recorder) WriteFrame(rgba []byte) error {
	if r.closed {
		return fmt.Errorf("recording is closed")
	}
	if len(rgba) != r.width*r.height*4 {
		return fmt.Errorf("native video frame has %d bytes, expected %d", len(rgba), r.width*r.height*4)
	}
	target := r.updates * int64(r.fps) / int64(r.tps)
	for r.frames < target {
		if _, err := r.video.Write(rgba); err != nil {
			return fmt.Errorf("encode native frame: %w", err)
		}
		r.frames++
	}
	return nil
}

// Duration returns the elapsed offline media time.
func (r *Recorder) Duration() time.Duration {
	return time.Duration(r.updates) * time.Second / time.Duration(r.tps)
}

// AddCue appends an English subtitle. Overlap is rejected during finalization.
func (r *Recorder) AddCue(start, end time.Duration, text string) error {
	if start < 0 || end <= start || strings.TrimSpace(text) == "" {
		return fmt.Errorf("invalid presentation subtitle")
	}
	r.cues = append(r.cues, Cue{start, end, strings.TrimSpace(text)})
	return nil
}

// Close drains H.264, finalizes a PCM WAV, and muxes AAC and English subtitles.
// The final MP4 replaces the destination only after successful encoding.
func (r *Recorder) Close() error {
	if r.closed {
		return r.closeError
	}
	r.closed = true
	r.closeError = r.finish()
	return r.closeError
}

func (r *Recorder) finish() error {
	inputErr := r.video.Close()
	encodeErr := r.encoder.Wait()
	if encodeErr != nil {
		r.audio.Close()
		return fmt.Errorf("finish video encoder: %w: %s", encodeErr, strings.TrimSpace(r.stderr.String()))
	}
	if inputErr != nil {
		r.audio.Close()
		return inputErr
	}
	if r.frames == 0 {
		r.audio.Close()
		return fmt.Errorf("recording contains no native video frames")
	}
	if r.frames != r.updates*int64(r.fps)/int64(r.tps) {
		r.audio.Close()
		return fmt.Errorf("final native render is missing; record the last due frame before closing")
	}
	if _, err := r.audio.Seek(0, io.SeekStart); err != nil {
		r.audio.Close()
		return err
	}
	if _, err := r.audio.Write(wavHeader(r.audioFrames)); err != nil {
		r.audio.Close()
		return err
	}
	if err := r.audio.Close(); err != nil {
		return err
	}
	base := strings.TrimSuffix(r.path, filepath.Ext(r.path))
	if err := os.Rename(r.audioPath, base+".wav"); err != nil {
		return err
	}
	r.audioPath = base + ".wav"
	srtPath := base + ".srt"
	if err := writeSRT(srtPath, r.cues, r.Duration()); err != nil {
		return err
	}
	output, err := os.CreateTemp(filepath.Dir(r.path), ".presentation-mux-*.mp4")
	if err != nil {
		return err
	}
	outputPath := output.Name()
	if err := output.Close(); err != nil {
		os.Remove(outputPath)
		return err
	}
	defer os.Remove(outputPath)
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", r.videoPath, "-i", r.audioPath}
	if len(r.cues) > 0 {
		args = append(args, "-i", srtPath)
	}
	args = append(args, "-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "aac", "-b:a", "160k")
	if len(r.cues) > 0 {
		args = append(args, "-map", "2:s:0", "-c:s", "mov_text", "-metadata:s:s:0", "language=eng", "-metadata:s:s:0", "title=English presentation", "-disposition:s:0", "default")
	}
	args = append(args, "-t", fmt.Sprintf("%.6f", r.Duration().Seconds()), "-movflags", "+faststart", outputPath)
	command := exec.Command(r.encoder.Path, args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("mux presentation audio and subtitles: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if err := os.Rename(outputPath, r.path); err != nil {
		return err
	}
	return os.Remove(r.videoPath)
}

func writeSRT(path string, cues []Cue, duration time.Duration) error {
	ordered := append([]Cue(nil), cues...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Start < ordered[j].Start })
	var output strings.Builder
	lastEnd := time.Duration(0)
	for i, cue := range ordered {
		if cue.Start < lastEnd {
			return errors.New("presentation subtitles overlap")
		}
		if cue.End > duration {
			cue.End = duration
		}
		if cue.End <= cue.Start {
			continue
		}
		fmt.Fprintf(&output, "%d\n%s --> %s\n%s\n\n", i+1, srtTime(cue.Start), srtTime(cue.End), cue.Text)
		lastEnd = cue.End
	}
	return os.WriteFile(path, []byte(output.String()), 0644)
}

func srtTime(value time.Duration) string {
	milliseconds := value.Milliseconds()
	return fmt.Sprintf("%02d:%02d:%02d,%03d", milliseconds/3600000, milliseconds/60000%60, milliseconds/1000%60, milliseconds%1000)
}

func wavHeader(frames int64) []byte {
	data := make([]byte, 44)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(frames*4+36))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 2)
	binary.LittleEndian.PutUint32(data[24:], SampleRate)
	binary.LittleEndian.PutUint32(data[28:], SampleRate*4)
	binary.LittleEndian.PutUint16(data[32:], 4)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(frames*4))
	return data
}
