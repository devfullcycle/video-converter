package encoder

import (
	"slices"
	"strings"
	"testing"
)

func TestBuildArgsCPU_H264(t *testing.T) {
	got, err := BuildArgs(Spec{
		Backend: BackendCPU, Codec: CodecH264, Quality: 60, Intensity: IntensityBalanced,
		InputPath: "in.mkv", OutputPath: "out.mp4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-i", "in.mkv",
		"-c:v", "libx264", "-crf", "29", "-preset", "medium",
		"-c:a", "copy",
		"-movflags", "+faststart",
		"out.mp4",
	}
	if !slices.Equal(got, want) {
		t.Errorf("args mismatch:\n got=%v\nwant=%v", got, want)
	}
}

func TestBuildArgsNVENC_OrderAndHwaccelBeforeInput(t *testing.T) {
	got, err := BuildArgs(Spec{
		Backend: BackendNVENC, Codec: CodecH264, Quality: 60, Intensity: IntensityMax,
		InputPath: "in.mp4", OutputPath: "out.mp4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	iHw := slices.Index(got, "cuda")
	iInput := slices.Index(got, "-i")
	if iHw == -1 || iInput == -1 || iHw > iInput {
		t.Errorf("hwaccel must come before -i; got %v", got)
	}
	for _, want := range []string{"h264_nvenc", "-rc", "vbr", "-cq", "-b:v", "0", "-preset", "p7", "-tune", "hq"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
}

func TestBuildArgsVAAPI_UsesDeviceAndHwupload(t *testing.T) {
	got, err := BuildArgs(Spec{
		Backend: BackendVAAPI, Codec: CodecH264, Quality: 50, Intensity: IntensityBalanced,
		InputPath: "in.mp4", OutputPath: "out.mkv", VAAPIDevice: "/dev/dri/renderD129",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Contains(got, "/dev/dri/renderD129") {
		t.Errorf("custom vaapi device not used: %v", got)
	}
	if !slices.Contains(got, "format=nv12,hwupload") {
		t.Errorf("missing hwupload filter: %v", got)
	}
	// .mkv must NOT get faststart.
	if slices.Contains(got, "+faststart") {
		t.Errorf("mkv output should not include faststart: %v", got)
	}
}

func TestBuildArgsAppliesScaleAndFPS(t *testing.T) {
	got, err := BuildArgs(Spec{
		Backend: BackendCPU, Codec: CodecH264, Quality: 60, Intensity: IntensityBalanced,
		FPS: 30, MaxHeight: 1080,
		InputPath: "in.mkv", OutputPath: "out.mp4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	i := slices.Index(got, "-vf")
	if i == -1 || i+1 >= len(got) {
		t.Fatalf("missing -vf in %v", got)
	}
	want := "scale=-2:min(1080\\,ih),fps=30"
	if got[i+1] != want {
		t.Errorf("filter chain = %q, want %q", got[i+1], want)
	}
	// The filter must precede the codec.
	if i > slices.Index(got, "-c:v") {
		t.Errorf("-vf must come before -c:v; got %v", got)
	}
}

func TestBuildArgsNoFiltersWhenDisabled(t *testing.T) {
	// FPS/MaxHeight default to 0 (keep source) -> no -vf at all. This is also the
	// shape the functional probe relies on.
	got, err := BuildArgs(Spec{
		Backend: BackendNVENC, Codec: CodecH264, Quality: 60, Intensity: IntensityBalanced,
		InputPath: "in.mp4", OutputPath: "out.mp4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slices.Contains(got, "-vf") {
		t.Errorf("expected no -vf when fps/scale disabled; got %v", got)
	}
}

func TestBuildArgsVAAPIMergesFiltersBeforeHwupload(t *testing.T) {
	got, err := BuildArgs(Spec{
		Backend: BackendVAAPI, Codec: CodecH264, Quality: 50, Intensity: IntensityBalanced,
		FPS: 24, MaxHeight: 720,
		InputPath: "in.mp4", OutputPath: "out.mp4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	i := slices.Index(got, "-vf")
	if i == -1 || i+1 >= len(got) {
		t.Fatalf("missing -vf in %v", got)
	}
	want := "scale=-2:min(720\\,ih),fps=24,format=nv12,hwupload"
	if got[i+1] != want {
		t.Errorf("vaapi filter chain = %q, want %q", got[i+1], want)
	}
}

func TestBuildArgsRejectsUnsupportedCodec(t *testing.T) {
	_, err := BuildArgs(Spec{
		Backend: BackendVideoToolbox, Codec: CodecAV1,
		InputPath: "in.mp4", OutputPath: "out.mp4",
	})
	if err == nil || !strings.Contains(err.Error(), "does not support codec") {
		t.Errorf("expected unsupported-codec error, got %v", err)
	}
}

func TestQualityMappingMonotonicAndClamped(t *testing.T) {
	if qpFromQuality(0) != 51 {
		t.Errorf("qp(0) = %d, want 51", qpFromQuality(0))
	}
	if got := qpFromQuality(100); got != 15 {
		t.Errorf("qp(100) = %d, want 15", got)
	}
	// out-of-range clamps
	if qpFromQuality(-10) != 51 || qpFromQuality(200) != 15 {
		t.Errorf("qp clamp failed: %d %d", qpFromQuality(-10), qpFromQuality(200))
	}
	// monotonic non-increasing as quality rises
	prev := 100
	for q := 0; q <= 100; q++ {
		v := qpFromQuality(q)
		if v > prev {
			t.Fatalf("qp not monotonic at q=%d (%d > %d)", q, v, prev)
		}
		prev = v
	}
	// AV1 wider range, VideoToolbox identity-ish
	if av1CrfFromQuality(0) != 63 {
		t.Errorf("av1 crf(0) = %d, want 63", av1CrfFromQuality(0))
	}
	if vtQualityFromQuality(0) != 1 || vtQualityFromQuality(60) != 60 {
		t.Errorf("vt mapping wrong: %d %d", vtQualityFromQuality(0), vtQualityFromQuality(60))
	}
}
