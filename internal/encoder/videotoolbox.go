package encoder

// videotoolbox is the Apple hardware backend (macOS). It supports H.264 and
// HEVC but not AV1. Constant-quality (-q:v) only applies on Apple Silicon with
// ffmpeg >= 4.4; on other Macs ffmpeg falls back to its default rate control.
// -allow_sw 1 lets ffmpeg fall back to software if the hardware path is
// unavailable, which keeps the run from failing outright.
type videotoolbox struct{}

func (videotoolbox) Name() Backend       { return BackendVideoToolbox }
func (videotoolbox) Platforms() []string { return []string{"darwin"} }
func (videotoolbox) Supports(c Codec) bool {
	return c == CodecH264 || c == CodecHEVC
}

func (videotoolbox) EncoderID(c Codec) string {
	switch c {
	case CodecH264:
		return "h264_videotoolbox"
	case CodecHEVC:
		return "hevc_videotoolbox"
	}
	return ""
}

func (videotoolbox) DeviceArgs(s Spec) []string { return nil }
func (videotoolbox) DecodeArgs(s Spec) []string { return []string{"-hwaccel", "videotoolbox"} }

func (b videotoolbox) OutputArgs(s Spec) []string {
	return append(vfArgs(s),
		"-c:v", b.EncoderID(s.Codec),
		"-q:v", itoa(vtQualityFromQuality(s.Quality)),
		"-allow_sw", "1",
	)
}
