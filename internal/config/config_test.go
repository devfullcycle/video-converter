package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devfullcycle/video-converter/internal/encoder"
)

func baseArgs() []string { return []string{"-input", "in", "-output", "out"} }

func TestDefaults(t *testing.T) {
	clearEnv(t)
	cfg, err := Load("", baseArgs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Backend != encoder.BackendAuto || cfg.Codec != encoder.CodecH264 || cfg.Quality != 60 {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.Intensity != encoder.IntensityBalanced || !cfg.FallbackOnFailure {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.FPS != 30 || cfg.MaxHeight != 1080 {
		t.Errorf("unexpected fps/scale defaults: fps=%d scale=%d", cfg.FPS, cfg.MaxHeight)
	}
}

func TestFPSAndScalePrecedence(t *testing.T) {
	clearEnv(t)
	t.Setenv("FPS", "20")
	t.Setenv("SCALE", "720")

	// env overrides default
	cfg, err := Load("", baseArgs())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cfg.FPS != 20 || cfg.MaxHeight != 720 {
		t.Errorf("env did not apply: fps=%d scale=%d", cfg.FPS, cfg.MaxHeight)
	}

	// flag overrides env; 0 disables (keep source)
	cfg, err = Load("", append(baseArgs(), "-fps", "0", "-scale", "1440"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cfg.FPS != 0 || cfg.MaxHeight != 1440 {
		t.Errorf("flag did not override env: fps=%d scale=%d", cfg.FPS, cfg.MaxHeight)
	}
}

func TestPrecedenceFlagOverEnvOverDefault(t *testing.T) {
	clearEnv(t)
	t.Setenv("QUALITY", "40")
	t.Setenv("BACKEND", "nvenc")

	// env overrides default
	cfg, err := Load("", baseArgs())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cfg.Quality != 40 || cfg.Backend != encoder.BackendNVENC {
		t.Errorf("env did not apply: %+v", cfg)
	}

	// flag overrides env
	cfg, err = Load("", append(baseArgs(), "-quality", "90", "-backend", "cpu"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cfg.Quality != 90 || cfg.Backend != encoder.BackendCPU {
		t.Errorf("flag did not override env: %+v", cfg)
	}
}

func TestDotEnvDoesNotClobberRealEnv(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("QUALITY=10\nBACKEND=qsv\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Real env set -> .env must NOT override it.
	t.Setenv("QUALITY", "40")
	cfg, err := Load(envFile, baseArgs())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cfg.Quality != 40 {
		t.Errorf("real env should win over .env, got quality=%d", cfg.Quality)
	}
	// BACKEND wasn't set in the real env, so .env should apply.
	if cfg.Backend != encoder.BackendQSV {
		t.Errorf(".env should apply for unset keys, got backend=%s", cfg.Backend)
	}
}

func TestExtensionsAndOverwriteParsing(t *testing.T) {
	clearEnv(t)
	cfg, err := Load("", append(baseArgs(), "-ext", "MP4, mov ,webm", "-overwrite"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []string{".mp4", ".mov", ".webm"}
	if len(cfg.Extensions) != 3 {
		t.Fatalf("extensions = %v", cfg.Extensions)
	}
	for i, e := range want {
		if cfg.Extensions[i] != e {
			t.Errorf("ext[%d] = %q want %q", i, cfg.Extensions[i], e)
		}
	}
	if !cfg.Overwrite {
		t.Error("overwrite flag not applied")
	}
}

func TestValidationErrors(t *testing.T) {
	clearEnv(t)
	if _, err := Load("", nil); err == nil {
		t.Error("expected error when input/output missing")
	}
	if _, err := Load("", append(baseArgs(), "-codec", "vp9")); err == nil {
		t.Error("expected error for invalid codec")
	}
	if _, err := Load("", append(baseArgs(), "-backend", "bogus")); err == nil {
		t.Error("expected error for invalid backend")
	}
}

// clearEnv truly unsets every config-relevant env var for the duration of a
// test, restoring the original value afterwards.
func clearEnv(t *testing.T) {
	for _, k := range []string{
		"INPUT_DIR", "OUTPUT_DIR", "BACKEND", "CODEC", "QUALITY", "FPS", "SCALE",
		"INTENSITY", "WORKERS", "THREADS", "EXTENSIONS", "OVERWRITE",
		"FALLBACK_ON_FAILURE", "VAAPI_DEVICE", "NICE", "LOG_DIR",
	} {
		orig, had := os.LookupEnv(k)
		os.Unsetenv(k)
		if had {
			t.Cleanup(func() { os.Setenv(k, orig) })
		} else {
			t.Cleanup(func() { os.Unsetenv(k) })
		}
	}
}
