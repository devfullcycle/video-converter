package encoder

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// probeTimeout bounds a single functional probe so a hung driver can't stall
// startup.
const probeTimeout = 15 * time.Second

// probeBackend runs a real one-frame encode against a synthetic source to prove
// the encoder actually works on this machine. Presence in `ffmpeg -encoders` is
// necessary but not sufficient: a build can advertise h264_nvenc while no GPU,
// driver, or VAAPI device is present. This is the single most important
// robustness feature — it's what makes "auto" reliably fall back to CPU.
func probeBackend(ctx context.Context, ffmpeg string, be EncoderBackend, codec Codec, vaapiDevice string) error {
	pctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	spec := Spec{
		Backend:     be.Name(),
		Codec:       codec,
		Quality:     60,
		Intensity:   IntensityBalanced,
		VAAPIDevice: vaapiDevice,
	}

	// Probe only the encoder: include device init (e.g. -vaapi_device) but NOT
	// decode hwaccel, since the source is synthetic software frames.
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	args = append(args, be.DeviceArgs(spec)...)
	// 320x240 is comfortably above the minimum frame size some hardware
	// encoders (notably NVENC) require.
	args = append(args, "-f", "lavfi", "-i", "color=c=black:s=320x240:r=5:d=0.2")
	args = append(args, be.OutputArgs(spec)...)
	args = append(args, "-frames:v", "1", "-an", "-f", "null", "-")

	cmd := exec.CommandContext(pctx, ffmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if pctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("probe timed out after %s", probeTimeout)
		}
		return fmt.Errorf("%v: %s", err, lastLines(stderr.String(), 2))
	}
	return nil
}

// lastLines returns the last n non-empty lines of s, joined with "; ", for
// compact error reporting.
func lastLines(s string, n int) string {
	raw := strings.Split(strings.TrimSpace(s), "\n")
	var lines []string
	for _, l := range raw {
		if t := strings.TrimSpace(l); t != "" {
			lines = append(lines, t)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "; ")
}
