package convert

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/devfullcycle/video-converter/internal/encoder"
)

// TestRunCPUEndToEnd generates a tiny clip with ffmpeg and converts it with the
// CPU backend, asserting the output exists and the temp file is gone. It is
// skipped in -short mode or when ffmpeg is unavailable. CPU encoding is always
// available, so this runs on any CI with ffmpeg installed.
func TestRunCPUEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp4")
	gen := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=128x128:rate=10",
		"-pix_fmt", "yuv420p", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generating test clip: %v: %s", err, out)
	}

	outBase := filepath.Join(dir, "out")
	jobs := []Job{{
		Input:  src,
		Output: filepath.Join(outBase, "src.mp4"),
		Temp:   filepath.Join(outBase, "src.part.mp4"),
		Rel:    "src.mp4",
	}}

	results, err := Run(context.Background(), jobs, Options{
		FFmpeg:    ffmpeg,
		Backend:   encoder.BackendCPU,
		Codec:     encoder.CodecH264,
		Quality:   60,
		Intensity: encoder.IntensityLight,
		Workers:   1,
		LogDir:    filepath.Join(dir, "logs"),
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(results) != 1 || results[0].Status != StatusOK {
		t.Fatalf("unexpected results: %+v", results)
	}

	if fi, err := os.Stat(jobs[0].Output); err != nil || fi.Size() == 0 {
		t.Fatalf("output missing or empty: %v", err)
	}
	if _, err := os.Stat(jobs[0].Temp); !os.IsNotExist(err) {
		t.Errorf("temp .part file should have been removed")
	}

	// Re-run should skip the existing output.
	results, _ = Run(context.Background(), jobs, Options{
		FFmpeg: ffmpeg, Backend: encoder.BackendCPU, Codec: encoder.CodecH264,
		Quality: 60, Intensity: encoder.IntensityLight, Workers: 1,
	})
	if results[0].Status != StatusSkip {
		t.Errorf("expected skip on re-run, got %s", results[0].Status)
	}
}
