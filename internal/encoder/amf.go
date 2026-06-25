package encoder

// amf is the AMD Advanced Media Framework backend (Windows). On Linux, AMD GPUs
// are driven through VAAPI instead, so this backend is Windows-only.
type amf struct{}

func (amf) Name() Backend       { return BackendAMF }
func (amf) Platforms() []string { return []string{"windows"} }
func (amf) Supports(c Codec) bool {
	return c == CodecH264 || c == CodecHEVC || c == CodecAV1
}

func (amf) EncoderID(c Codec) string {
	switch c {
	case CodecH264:
		return "h264_amf"
	case CodecHEVC:
		return "hevc_amf"
	case CodecAV1:
		return "av1_amf"
	}
	return ""
}

func (amf) DeviceArgs(s Spec) []string { return nil }
func (amf) DecodeArgs(s Spec) []string { return []string{"-hwaccel", "d3d11va"} }

func (b amf) OutputArgs(s Spec) []string {
	qp := qpFromQuality(s.Quality)
	return append(vfArgs(s),
		"-c:v", b.EncoderID(s.Codec),
		"-rc", "cqp",
		"-qp_i", itoa(qp),
		"-qp_p", itoa(clampInt(qp+3, 0, 51)),
		"-quality", amfQuality(s.Intensity),
	)
}

// amfQuality maps intensity onto AMF's quality preset.
func amfQuality(i Intensity) string {
	switch i {
	case IntensityLight:
		return "speed"
	case IntensityMax:
		return "quality"
	default:
		return "balanced"
	}
}
