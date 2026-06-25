package encoder

// qsv is the Intel Quick Sync Video backend. It is the best choice on machines
// with no discrete GPU (integrated Intel graphics, low power draw).
type qsv struct{}

func (qsv) Name() Backend       { return BackendQSV }
func (qsv) Platforms() []string { return []string{"linux", "windows"} }
func (qsv) Supports(c Codec) bool {
	return c == CodecH264 || c == CodecHEVC || c == CodecAV1
}

func (qsv) EncoderID(c Codec) string {
	switch c {
	case CodecH264:
		return "h264_qsv"
	case CodecHEVC:
		return "hevc_qsv"
	case CodecAV1:
		return "av1_qsv"
	}
	return ""
}

func (qsv) DeviceArgs(s Spec) []string { return nil }
func (qsv) DecodeArgs(s Spec) []string { return []string{"-hwaccel", "qsv"} }

func (b qsv) OutputArgs(s Spec) []string {
	args := append(vfArgs(s),
		"-c:v", b.EncoderID(s.Codec),
		"-global_quality", itoa(qpFromQuality(s.Quality)),
		"-preset", qsvPreset(s.Intensity),
	)
	// -look_ahead (LA_ICQ) is reliably supported on h264_qsv; some builds
	// reject it on hevc/av1, so only enable it for H.264.
	if s.Codec == CodecH264 {
		args = append(args, "-look_ahead", "1")
	}
	return args
}

// qsvPreset maps intensity onto QSV presets (veryfast .. veryslow).
func qsvPreset(i Intensity) string {
	switch i {
	case IntensityLight:
		return "veryfast"
	case IntensityMax:
		return "veryslow"
	default:
		return "medium"
	}
}
