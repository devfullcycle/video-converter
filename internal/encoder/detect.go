package encoder

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Environment captures everything we learned about the host's ffmpeg: where the
// binaries are and which encoders the build advertises. Prober is injectable so
// selection can be unit-tested without invoking ffmpeg.
type Environment struct {
	FFmpegPath  string
	FFprobePath string // may be empty if ffprobe is not installed
	Encoders    map[string]bool
	Prober      func(ctx context.Context, be EncoderBackend, codec Codec, vaapiDevice string) error
}

// DetectEnvironment locates ffmpeg/ffprobe and lists the available encoders.
// It returns a clear error if ffmpeg is missing.
func DetectEnvironment(ctx context.Context) (*Environment, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH — install it: https://ffmpeg.org/download.html")
	}
	ffprobe, _ := exec.LookPath("ffprobe") // optional, used only for nicer reports

	encs, err := listEncoders(ctx, ffmpeg)
	if err != nil {
		return nil, err
	}

	env := &Environment{FFmpegPath: ffmpeg, FFprobePath: ffprobe, Encoders: encs}
	env.Prober = func(ctx context.Context, be EncoderBackend, codec Codec, dev string) error {
		return probeBackend(ctx, ffmpeg, be, codec, dev)
	}
	return env, nil
}

// listEncoders parses `ffmpeg -encoders` into a set of encoder names.
// Each data line looks like: " V....D h264_nvenc  NVIDIA NVENC H.264 encoder".
func listEncoders(ctx context.Context, ffmpeg string) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-encoders")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing ffmpeg encoders: %w", err)
	}
	set := make(map[string]bool)
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// The capability column is exactly 6 chars (e.g. "V....D") and the
		// first char encodes the media type (V/A/S).
		if len(fields) >= 2 && len(fields[0]) == 6 && strings.ContainsAny(fields[0][:1], "VAS") {
			set[fields[1]] = true
		}
	}
	return set, nil
}
