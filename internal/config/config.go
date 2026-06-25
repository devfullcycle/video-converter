// Package config resolves the converter's settings from, in order of
// precedence: command-line flags > environment (including a .env file) >
// built-in defaults.
package config

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/devfullcycle/video-converter/internal/encoder"
)

// Config is the fully-resolved configuration.
type Config struct {
	InputDir          string
	OutputDir         string
	Backend           encoder.Backend
	Codec             encoder.Codec
	Quality           int
	FPS               int // cap output frame rate; 0 = keep source rate
	MaxHeight         int // cap output height (px); 0 = keep source size
	Intensity         encoder.Intensity
	Workers           int // 0 = auto (per-backend default)
	Threads           int // 0 = derived (CPU only)
	Extensions        []string
	Overwrite         bool
	FallbackOnFailure bool
	VAAPIDevice       string
	Nice              int // 0 = leave priority unchanged
	LogDir            string
}

// Defaults returns the built-in defaults.
func Defaults() Config {
	return Config{
		Backend:           encoder.BackendAuto,
		Codec:             encoder.CodecH264,
		Quality:           60,
		FPS:               30,
		MaxHeight:         1080,
		Intensity:         encoder.IntensityBalanced,
		Workers:           0,
		Threads:           0,
		Extensions:        []string{".mp4", ".mov", ".mkv", ".avi"},
		Overwrite:         false,
		FallbackOnFailure: true,
		VAAPIDevice:       "/dev/dri/renderD128",
		Nice:              0,
		LogDir:            "logs",
	}
}

// Load resolves configuration from a .env file (if present at envPath), the
// process environment, and the provided command-line args. Pass os.Args[1:] in
// production; tests pass a controlled slice.
func Load(envPath string, args []string) (*Config, error) {
	if err := loadDotEnv(envPath); err != nil {
		return nil, err
	}

	// base = defaults overlaid with environment values.
	base := Defaults()
	applyEnv(&base)

	// Flags default to the env-resolved base, so any flag left untouched keeps
	// the env/default value — giving flags > env > default automatically.
	fs := flag.NewFlagSet("video-converter", flag.ContinueOnError)
	input := fs.String("input", base.InputDir, "input directory containing videos (required)")
	output := fs.String("output", base.OutputDir, "output base directory (required)")
	backend := fs.String("backend", string(base.Backend), "encoder backend: auto|cpu|nvenc|qsv|amf|vaapi|videotoolbox")
	codec := fs.String("codec", string(base.Codec), "video codec: h264|hevc|av1")
	quality := fs.Int("quality", base.Quality, "quality 0..100 (higher = better)")
	fps := fs.Int("fps", base.FPS, "cap output frame rate, e.g. 30 (0 = keep source)")
	scale := fs.Int("scale", base.MaxHeight, "cap output height in px, e.g. 1080 (0 = keep source)")
	intensity := fs.String("intensity", string(base.Intensity), "light|balanced|max (speed vs machine usage)")
	workers := fs.Int("workers", base.Workers, "concurrent ffmpeg jobs (0 = auto per backend)")
	threads := fs.Int("threads", base.Threads, "ffmpeg CPU threads per job (0 = derived)")
	exts := fs.String("ext", strings.Join(stripDots(base.Extensions), ","), "comma-separated input extensions")
	overwrite := fs.Bool("overwrite", base.Overwrite, "overwrite existing outputs (default: skip)")
	fallback := fs.Bool("fallback", base.FallbackOnFailure, "fall back to CPU if the chosen backend fails")
	vaapiDev := fs.String("vaapi-device", base.VAAPIDevice, "VAAPI render node (Linux)")
	nice := fs.Int("nice", base.Nice, "scheduling priority for ffmpeg (0 = unchanged)")
	logDir := fs.String("log-dir", base.LogDir, "directory for per-file ffmpeg error logs (default: <output>/.logs)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg := Config{
		InputDir:          *input,
		OutputDir:         *output,
		Backend:           encoder.Backend(strings.ToLower(*backend)),
		Codec:             encoder.Codec(strings.ToLower(*codec)),
		Quality:           *quality,
		FPS:               *fps,
		MaxHeight:         *scale,
		Intensity:         encoder.Intensity(strings.ToLower(*intensity)),
		Workers:           *workers,
		Threads:           *threads,
		Extensions:        normalizeExts(splitCSV(*exts)),
		Overwrite:         *overwrite,
		FallbackOnFailure: *fallback,
		VAAPIDevice:       *vaapiDev,
		Nice:              *nice,
		LogDir:            *logDir,
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.InputDir == "" || c.OutputDir == "" {
		return fmt.Errorf("both -input and -output are required (or set INPUT_DIR/OUTPUT_DIR)")
	}
	if _, ok := encoder.Lookup(c.Backend); !ok && c.Backend != encoder.BackendAuto {
		return fmt.Errorf("invalid backend %q (valid: auto, %v)", c.Backend, encoder.AllBackends())
	}
	switch c.Codec {
	case encoder.CodecH264, encoder.CodecHEVC, encoder.CodecAV1:
	default:
		return fmt.Errorf("invalid codec %q (valid: h264, hevc, av1)", c.Codec)
	}
	switch c.Intensity {
	case encoder.IntensityLight, encoder.IntensityBalanced, encoder.IntensityMax:
	default:
		return fmt.Errorf("invalid intensity %q (valid: light, balanced, max)", c.Intensity)
	}
	c.Quality = clamp(c.Quality, 0, 100)
	if c.Workers < 0 || c.Threads < 0 {
		return fmt.Errorf("workers and threads must be >= 0")
	}
	if c.FPS < 0 || c.MaxHeight < 0 {
		return fmt.Errorf("fps and scale must be >= 0 (0 = keep source)")
	}
	if len(c.Extensions) == 0 {
		return fmt.Errorf("at least one input extension is required")
	}
	return nil
}

// applyEnv overlays environment variables onto cfg.
func applyEnv(cfg *Config) {
	envStr("INPUT_DIR", &cfg.InputDir)
	envStr("OUTPUT_DIR", &cfg.OutputDir)
	if v := os.Getenv("BACKEND"); v != "" {
		cfg.Backend = encoder.Backend(strings.ToLower(v))
	}
	if v := os.Getenv("CODEC"); v != "" {
		cfg.Codec = encoder.Codec(strings.ToLower(v))
	}
	envInt("QUALITY", &cfg.Quality)
	envInt("FPS", &cfg.FPS)
	envInt("SCALE", &cfg.MaxHeight)
	if v := os.Getenv("INTENSITY"); v != "" {
		cfg.Intensity = encoder.Intensity(strings.ToLower(v))
	}
	envInt("WORKERS", &cfg.Workers)
	envInt("THREADS", &cfg.Threads)
	if v := os.Getenv("EXTENSIONS"); v != "" {
		cfg.Extensions = normalizeExts(splitCSV(v))
	}
	if v := os.Getenv("OVERWRITE"); v != "" {
		cfg.Overwrite = truthy(v) || strings.EqualFold(v, "overwrite")
	}
	if v := os.Getenv("FALLBACK_ON_FAILURE"); v != "" {
		cfg.FallbackOnFailure = truthy(v)
	}
	envStr("VAAPI_DEVICE", &cfg.VAAPIDevice)
	envInt("NICE", &cfg.Nice)
	envStr("LOG_DIR", &cfg.LogDir)
}

// loadDotEnv reads KEY=VALUE lines from envPath into the process environment
// WITHOUT clobbering variables that are already set. A missing file is not an
// error.
func loadDotEnv(envPath string) error {
	if envPath == "" {
		return nil
	}
	f, err := os.Open(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", envPath, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
	return sc.Err()
}

// --- small helpers ---

func envStr(key string, dst *string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

func envInt(key string, dst *int) {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			*dst = n
		}
	}
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// normalizeExts lowercases and ensures a leading dot on each extension.
func normalizeExts(in []string) []string {
	out := make([]string, 0, len(in))
	for _, e := range in {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		out = append(out, e)
	}
	return out
}

func stripDots(in []string) []string {
	out := make([]string, len(in))
	for i, e := range in {
		out[i] = strings.TrimPrefix(e, ".")
	}
	return out
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
