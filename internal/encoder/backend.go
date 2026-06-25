// Package encoder builds ffmpeg argument lists for the different video
// encoding backends (CPU and the various hardware encoders), and selects a
// working backend at runtime.
//
// The central abstraction is the EncoderBackend interface: each backend knows
// which ffmpeg encoder to use, which decode/hwaccel flags to place before -i,
// and which encode flags to place after -i. BuildArgs assembles a full,
// shell-free argument slice from a fully-resolved Spec. All of this is pure
// (no ffmpeg calls), which makes it trivially unit-testable.
package encoder

import "runtime"

// Backend identifies an encoding backend.
type Backend string

const (
	BackendAuto         Backend = "auto"
	BackendCPU          Backend = "cpu"
	BackendNVENC        Backend = "nvenc"
	BackendQSV          Backend = "qsv"
	BackendAMF          Backend = "amf"
	BackendVAAPI        Backend = "vaapi"
	BackendVideoToolbox Backend = "videotoolbox"
)

// Codec identifies the target video codec.
type Codec string

const (
	CodecH264 Codec = "h264"
	CodecHEVC Codec = "hevc"
	CodecAV1  Codec = "av1"
)

// Intensity controls the speed/quality preset and how much of the machine the
// job is allowed to use. It is the "don't hog the machine" knob.
type Intensity string

const (
	IntensityLight    Intensity = "light"
	IntensityBalanced Intensity = "balanced"
	IntensityMax      Intensity = "max"
)

// Spec is a fully-resolved, immutable conversion request handed to a backend's
// argument builders. OutputPath is the path ffmpeg writes to (the runner passes
// a temporary ".part" path that keeps the final extension so ffmpeg can infer
// the muxer); its extension also drives container-specific flags.
type Spec struct {
	Backend     Backend
	Codec       Codec
	Quality     int // single 0..100 knob, higher = better visual quality
	Intensity   Intensity
	Threads     int // CPU -threads cap; 0 = ffmpeg default
	FPS         int // cap output frame rate to this value; 0 = keep source rate
	MaxHeight   int // cap output height (px), preserving aspect; 0 = keep source size
	InputPath   string
	OutputPath  string
	VAAPIDevice string // Linux render node, e.g. /dev/dri/renderD128
}

// EncoderBackend is the extension point. Adding a new hardware encoder means
// adding one file implementing this interface plus one entry in registry().
type EncoderBackend interface {
	// Name returns the backend identifier (e.g. BackendNVENC).
	Name() Backend
	// Platforms lists the runtime.GOOS values this backend can run on.
	Platforms() []string
	// Supports reports whether the backend can encode the given codec.
	Supports(c Codec) bool
	// EncoderID returns the ffmpeg encoder name for a codec (e.g. "h264_nvenc").
	// It must return "" for unsupported codecs.
	EncoderID(c Codec) string
	// DeviceArgs returns init flags placed BEFORE -i that the ENCODER itself
	// needs (e.g. -vaapi_device). They are used in both real runs and probes.
	DeviceArgs(s Spec) []string
	// DecodeArgs returns hardware-decode flags placed BEFORE -i for the real
	// input (e.g. -hwaccel cuda). They are used in real runs only — probes use a
	// synthetic source and must not apply decode acceleration.
	DecodeArgs(s Spec) []string
	// OutputArgs returns flags placed AFTER -i (codec, quality, preset, filters).
	OutputArgs(s Spec) []string
}

// registry lists the concrete backends in auto-detection priority order:
// strongest hardware first, CPU last as the guaranteed fallback.
func registry() []EncoderBackend {
	return []EncoderBackend{
		nvenc{},
		qsv{},
		amf{},
		vaapi{},
		videotoolbox{},
		cpu{},
	}
}

// Lookup returns the backend implementation for a Backend value.
func Lookup(b Backend) (EncoderBackend, bool) {
	for _, e := range registry() {
		if e.Name() == b {
			return e, true
		}
	}
	return nil, false
}

// AllBackends returns every selectable (non-auto) backend name.
func AllBackends() []Backend {
	out := make([]Backend, 0, len(registry()))
	for _, e := range registry() {
		out = append(out, e.Name())
	}
	return out
}

// supportsOS reports whether goos is in the platform list.
func supportsOS(platforms []string, goos string) bool {
	for _, p := range platforms {
		if p == goos {
			return true
		}
	}
	return false
}

// CurrentOS is overridable in tests; defaults to the build target's OS.
var CurrentOS = runtime.GOOS
