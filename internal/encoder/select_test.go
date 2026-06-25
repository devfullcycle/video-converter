package encoder

import (
	"context"
	"errors"
	"testing"
)

// fakeEnv builds an Environment with a controllable prober. failing holds the
// set of backends whose probe should fail.
func fakeEnv(encoders []string, failing map[Backend]bool) *Environment {
	set := make(map[string]bool, len(encoders))
	for _, e := range encoders {
		set[e] = true
	}
	return &Environment{
		FFmpegPath: "ffmpeg",
		Encoders:   set,
		Prober: func(_ context.Context, be EncoderBackend, _ Codec, _ string) error {
			if failing[be.Name()] {
				return errors.New("simulated probe failure")
			}
			return nil
		},
	}
}

func TestSelectAutoPrefersHardwareThenFallsBack(t *testing.T) {
	CurrentOS = "linux"
	defer func() { CurrentOS = "linux" }()

	// nvenc available and working -> auto should pick it (it's first in registry).
	env := fakeEnv([]string{"h264_nvenc", "h264_qsv", "libx264"}, nil)
	sel, err := Select(context.Background(), env, BackendAuto, CodecH264, true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Backend != BackendNVENC {
		t.Errorf("auto picked %s, want nvenc", sel.Backend)
	}

	// nvenc present but probe fails -> should skip to qsv.
	env = fakeEnv([]string{"h264_nvenc", "h264_qsv", "libx264"}, map[Backend]bool{BackendNVENC: true})
	sel, err = Select(context.Background(), env, BackendAuto, CodecH264, true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Backend != BackendQSV {
		t.Errorf("auto picked %s, want qsv after nvenc probe failure", sel.Backend)
	}

	// Nothing hardware -> CPU is the guaranteed fallback.
	env = fakeEnv([]string{"libx264"}, nil)
	sel, err = Select(context.Background(), env, BackendAuto, CodecH264, true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Backend != BackendCPU {
		t.Errorf("auto picked %s, want cpu", sel.Backend)
	}
}

func TestSelectExplicitFallbackPolicy(t *testing.T) {
	CurrentOS = "linux"
	defer func() { CurrentOS = "linux" }()

	// Explicit nvenc that fails, fallback=true -> CPU.
	env := fakeEnv([]string{"h264_nvenc", "libx264"}, map[Backend]bool{BackendNVENC: true})
	sel, err := Select(context.Background(), env, BackendNVENC, CodecH264, true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Backend != BackendCPU {
		t.Errorf("explicit-with-fallback picked %s, want cpu", sel.Backend)
	}

	// Explicit nvenc that fails, fallback=false -> error.
	env = fakeEnv([]string{"h264_nvenc", "libx264"}, map[Backend]bool{BackendNVENC: true})
	if _, err := Select(context.Background(), env, BackendNVENC, CodecH264, false, ""); err == nil {
		t.Error("expected error when explicit backend fails and fallback disabled")
	}
}

func TestSelectRespectsOS(t *testing.T) {
	CurrentOS = "darwin"
	defer func() { CurrentOS = "linux" }()

	// On macOS, nvenc/qsv are not eligible even if "present"; videotoolbox wins.
	env := fakeEnv([]string{"h264_nvenc", "h264_videotoolbox", "libx264"}, nil)
	sel, err := Select(context.Background(), env, BackendAuto, CodecH264, true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Backend != BackendVideoToolbox {
		t.Errorf("on darwin auto picked %s, want videotoolbox", sel.Backend)
	}
}
