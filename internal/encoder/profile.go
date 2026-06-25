package encoder

import (
	"fmt"
	"path/filepath"
	"strings"
)

// BuildArgs assembles the complete, shell-free ffmpeg argument slice for a
// fully-resolved Spec. It is pure: no ffmpeg is invoked and no filesystem is
// touched, so it is the primary unit-test target.
//
// Argument order is significant and fixed:
//
//	<global> <DeviceArgs> <DecodeArgs> -i <input>
//	<OutputArgs (codec/quality/preset)> <audio> <container> <output>
//
// We always pass -y because the runner writes to a fresh temporary ".part"
// file (overwriting a stale temp from a crashed run is correct); the
// skip-existing policy is enforced on the final path by the runner, never by an
// interactive ffmpeg prompt. -nostdin is an extra guard so ffmpeg can never
// block waiting for input.
func BuildArgs(s Spec) ([]string, error) {
	be, ok := Lookup(s.Backend)
	if !ok {
		return nil, fmt.Errorf("unknown backend %q", s.Backend)
	}
	if !be.Supports(s.Codec) {
		return nil, fmt.Errorf("backend %q does not support codec %q", s.Backend, s.Codec)
	}
	if s.InputPath == "" || s.OutputPath == "" {
		return nil, fmt.Errorf("input and output paths are required")
	}

	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y"}
	args = append(args, be.DeviceArgs(s)...)
	args = append(args, be.DecodeArgs(s)...)
	args = append(args, "-i", s.InputPath)
	args = append(args, be.OutputArgs(s)...)

	// Audio: copy the original stream (fast, lossless). This matches the
	// hardware-accel goal of offloading only video work.
	args = append(args, "-c:a", "copy")

	// Container-specific flags: web-friendly faststart for MP4/MOV.
	switch strings.ToLower(filepath.Ext(s.OutputPath)) {
	case ".mp4", ".mov", ".m4v":
		args = append(args, "-movflags", "+faststart")
	}

	args = append(args, s.OutputPath)
	return args, nil
}

// ContainerForCodec returns the output container extension that best fits a
// codec. H.264/HEVC/AV1 all live happily in MP4, which maximizes player
// compatibility, so we normalize to .mp4.
func ContainerForCodec(c Codec) string {
	return ".mp4"
}
