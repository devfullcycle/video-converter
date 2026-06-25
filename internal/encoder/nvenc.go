package encoder

// nvenc is the NVIDIA hardware backend (h264_nvenc/hevc_nvenc/av1_nvenc).
// AV1 requires Ada/Blackwell-class GPUs; the functional probe filters out
// unsupported combinations at runtime.
type nvenc struct{}

func (nvenc) Name() Backend       { return BackendNVENC }
func (nvenc) Platforms() []string { return []string{"linux", "windows"} }
func (nvenc) Supports(c Codec) bool {
	return c == CodecH264 || c == CodecHEVC || c == CodecAV1
}

func (nvenc) EncoderID(c Codec) string {
	switch c {
	case CodecH264:
		return "h264_nvenc"
	case CodecHEVC:
		return "hevc_nvenc"
	case CodecAV1:
		return "av1_nvenc"
	}
	return ""
}

func (nvenc) DeviceArgs(s Spec) []string { return nil }

// DecodeArgs enables CUDA hardware decode for a full GPU pipeline on real input.
func (nvenc) DecodeArgs(s Spec) []string { return []string{"-hwaccel", "cuda"} }

func (b nvenc) OutputArgs(s Spec) []string {
	// VBR + -cq + -b:v 0 is the canonical NVENC constant-quality mode.
	// -tune hq is supported across encoders/ffmpeg versions (uhq is newer and
	// intentionally avoided for compatibility).
	return append(vfArgs(s),
		"-c:v", b.EncoderID(s.Codec),
		"-rc", "vbr",
		"-cq", itoa(qpFromQuality(s.Quality)),
		"-b:v", "0",
		"-preset", nvencPreset(s.Intensity),
		"-tune", "hq",
	)
}

// nvencPreset maps intensity onto NVENC presets p1 (fastest) .. p7 (best).
func nvencPreset(i Intensity) string {
	switch i {
	case IntensityLight:
		return "p4"
	case IntensityMax:
		return "p7"
	default:
		return "p5"
	}
}
