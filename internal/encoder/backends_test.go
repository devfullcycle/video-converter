package encoder

import (
	"slices"
	"testing"
)

// full builds the expected full argument slice for an input "in.mp4" and output
// "out.mp4" (which gets faststart). `pre` is the device+decode args before -i;
// `mid` is the encoder's OutputArgs. The fixed global order is anchored
// separately by the literal assertions in profile_test.go.
func full(pre, mid []string) []string {
	out := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y"}
	out = append(out, pre...)
	out = append(out, "-i", "in.mp4")
	out = append(out, mid...)
	out = append(out, "-c:a", "copy", "-movflags", "+faststart", "out.mp4")
	return out
}

// TestBuildArgsMatrix locks the exact ffmpeg command for every supported
// (backend × codec) combination at QUALITY=60 / balanced. This is what lets us
// trust the macOS/AMD/Intel command shapes without owning that hardware.
func TestBuildArgsMatrix(t *testing.T) {
	const q = 60 // -> qp/cq/global_quality 29, av1 crf 37, vt q:v 60

	cases := []struct {
		name    string
		backend Backend
		codec   Codec
		want    []string
	}{
		// CPU (software)
		{"cpu/h264", BackendCPU, CodecH264, full(nil, []string{"-c:v", "libx264", "-crf", "29", "-preset", "medium"})},
		{"cpu/hevc", BackendCPU, CodecHEVC, full(nil, []string{"-c:v", "libx265", "-crf", "29", "-preset", "medium"})},
		{"cpu/av1", BackendCPU, CodecAV1, full(nil, []string{"-c:v", "libsvtav1", "-crf", "37", "-preset", "8"})},

		// NVIDIA NVENC
		{"nvenc/h264", BackendNVENC, CodecH264, full([]string{"-hwaccel", "cuda"}, []string{"-c:v", "h264_nvenc", "-rc", "vbr", "-cq", "29", "-b:v", "0", "-preset", "p5", "-tune", "hq"})},
		{"nvenc/hevc", BackendNVENC, CodecHEVC, full([]string{"-hwaccel", "cuda"}, []string{"-c:v", "hevc_nvenc", "-rc", "vbr", "-cq", "29", "-b:v", "0", "-preset", "p5", "-tune", "hq"})},
		{"nvenc/av1", BackendNVENC, CodecAV1, full([]string{"-hwaccel", "cuda"}, []string{"-c:v", "av1_nvenc", "-rc", "vbr", "-cq", "29", "-b:v", "0", "-preset", "p5", "-tune", "hq"})},

		// Intel QSV (-look_ahead only on H.264)
		{"qsv/h264", BackendQSV, CodecH264, full([]string{"-hwaccel", "qsv"}, []string{"-c:v", "h264_qsv", "-global_quality", "29", "-preset", "medium", "-look_ahead", "1"})},
		{"qsv/hevc", BackendQSV, CodecHEVC, full([]string{"-hwaccel", "qsv"}, []string{"-c:v", "hevc_qsv", "-global_quality", "29", "-preset", "medium"})},
		{"qsv/av1", BackendQSV, CodecAV1, full([]string{"-hwaccel", "qsv"}, []string{"-c:v", "av1_qsv", "-global_quality", "29", "-preset", "medium"})},

		// AMD AMF (qp_p = qp+3)
		{"amf/h264", BackendAMF, CodecH264, full([]string{"-hwaccel", "d3d11va"}, []string{"-c:v", "h264_amf", "-rc", "cqp", "-qp_i", "29", "-qp_p", "32", "-quality", "balanced"})},
		{"amf/hevc", BackendAMF, CodecHEVC, full([]string{"-hwaccel", "d3d11va"}, []string{"-c:v", "hevc_amf", "-rc", "cqp", "-qp_i", "29", "-qp_p", "32", "-quality", "balanced"})},
		{"amf/av1", BackendAMF, CodecAV1, full([]string{"-hwaccel", "d3d11va"}, []string{"-c:v", "av1_amf", "-rc", "cqp", "-qp_i", "29", "-qp_p", "32", "-quality", "balanced"})},

		// VAAPI (device init before -i; default render node; sw decode + hwupload)
		{"vaapi/h264", BackendVAAPI, CodecH264, full([]string{"-vaapi_device", "/dev/dri/renderD128"}, []string{"-vf", "format=nv12,hwupload", "-c:v", "h264_vaapi", "-qp", "29"})},
		{"vaapi/hevc", BackendVAAPI, CodecHEVC, full([]string{"-vaapi_device", "/dev/dri/renderD128"}, []string{"-vf", "format=nv12,hwupload", "-c:v", "hevc_vaapi", "-qp", "29"})},
		{"vaapi/av1", BackendVAAPI, CodecAV1, full([]string{"-vaapi_device", "/dev/dri/renderD128"}, []string{"-vf", "format=nv12,hwupload", "-c:v", "av1_vaapi", "-qp", "29"})},

		// Apple VideoToolbox (no AV1; -allow_sw guards against hard failure)
		{"videotoolbox/h264", BackendVideoToolbox, CodecH264, full([]string{"-hwaccel", "videotoolbox"}, []string{"-c:v", "h264_videotoolbox", "-q:v", "60", "-allow_sw", "1"})},
		{"videotoolbox/hevc", BackendVideoToolbox, CodecHEVC, full([]string{"-hwaccel", "videotoolbox"}, []string{"-c:v", "hevc_videotoolbox", "-q:v", "60", "-allow_sw", "1"})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildArgs(Spec{
				Backend: tc.backend, Codec: tc.codec, Quality: q, Intensity: IntensityBalanced,
				InputPath: "in.mp4", OutputPath: "out.mp4",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("args mismatch:\n got=%v\nwant=%v", got, tc.want)
			}
		})
	}
}

// TestEncoderIDAndSupports verifies the codec capability matrix, including the
// VideoToolbox AV1 hole.
func TestEncoderIDAndSupports(t *testing.T) {
	want := map[Backend]map[Codec]string{
		BackendCPU:          {CodecH264: "libx264", CodecHEVC: "libx265", CodecAV1: "libsvtav1"},
		BackendNVENC:        {CodecH264: "h264_nvenc", CodecHEVC: "hevc_nvenc", CodecAV1: "av1_nvenc"},
		BackendQSV:          {CodecH264: "h264_qsv", CodecHEVC: "hevc_qsv", CodecAV1: "av1_qsv"},
		BackendAMF:          {CodecH264: "h264_amf", CodecHEVC: "hevc_amf", CodecAV1: "av1_amf"},
		BackendVAAPI:        {CodecH264: "h264_vaapi", CodecHEVC: "hevc_vaapi", CodecAV1: "av1_vaapi"},
		BackendVideoToolbox: {CodecH264: "h264_videotoolbox", CodecHEVC: "hevc_videotoolbox", CodecAV1: ""},
	}

	for backend, byCodec := range want {
		be, ok := Lookup(backend)
		if !ok {
			t.Fatalf("backend %q not in registry", backend)
		}
		for codec, id := range byCodec {
			if got := be.EncoderID(codec); got != id {
				t.Errorf("%s.EncoderID(%s) = %q, want %q", backend, codec, got, id)
			}
			wantSupported := id != ""
			if be.Supports(codec) != wantSupported {
				t.Errorf("%s.Supports(%s) = %v, want %v", backend, codec, be.Supports(codec), wantSupported)
			}
		}
	}

	// VideoToolbox + AV1 must surface a clear error through BuildArgs.
	if _, err := BuildArgs(Spec{Backend: BackendVideoToolbox, Codec: CodecAV1, InputPath: "in.mp4", OutputPath: "out.mp4"}); err == nil {
		t.Error("expected error for videotoolbox + av1")
	}
}

// TestIntensityPresets locks the speed/quality preset chosen per backend for
// each intensity level.
func TestIntensityPresets(t *testing.T) {
	check := func(name string, fn func(Intensity) string, light, balanced, max string) {
		if got := fn(IntensityLight); got != light {
			t.Errorf("%s light = %q, want %q", name, got, light)
		}
		if got := fn(IntensityBalanced); got != balanced {
			t.Errorf("%s balanced = %q, want %q", name, got, balanced)
		}
		if got := fn(IntensityMax); got != max {
			t.Errorf("%s max = %q, want %q", name, got, max)
		}
	}
	check("x26x", x26xPreset, "veryfast", "medium", "slow")
	check("svtav1", svtAV1Preset, "10", "8", "6")
	check("nvenc", nvencPreset, "p4", "p5", "p7")
	check("qsv", qsvPreset, "veryfast", "medium", "veryslow")
	check("amf", amfQuality, "speed", "balanced", "quality")
}

// TestThreadsAppendedForCPU verifies the CPU backend honors the -threads cap.
func TestThreadsAppendedForCPU(t *testing.T) {
	got, err := BuildArgs(Spec{
		Backend: BackendCPU, Codec: CodecH264, Quality: 60, Intensity: IntensityBalanced,
		Threads: 4, InputPath: "in.mp4", OutputPath: "out.mp4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Contains(got, "-threads") || !slices.Contains(got, "4") {
		t.Errorf("expected -threads 4 in %v", got)
	}
}

// TestContainerFaststart confirms faststart is added only for MP4-family
// containers.
func TestContainerFaststart(t *testing.T) {
	cases := map[string]bool{"out.mp4": true, "out.mov": true, "out.m4v": true, "out.mkv": false, "out.webm": false}
	for outPath, wantFaststart := range cases {
		got, err := BuildArgs(Spec{
			Backend: BackendCPU, Codec: CodecH264, Quality: 60, Intensity: IntensityBalanced,
			InputPath: "in.mp4", OutputPath: outPath,
		})
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", outPath, err)
		}
		hasFaststart := slices.Contains(got, "+faststart")
		if hasFaststart != wantFaststart {
			t.Errorf("%s: faststart=%v, want %v", outPath, hasFaststart, wantFaststart)
		}
	}
}
