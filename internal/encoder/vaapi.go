package encoder

import "strings"

// vaapi is the Linux VAAPI backend, used for AMD and Intel GPUs on Linux.
// We use the robust software-decode + hwupload path (rather than full GPU
// decode) because it works across drivers without format-negotiation surprises.
type vaapi struct{}

func (vaapi) Name() Backend       { return BackendVAAPI }
func (vaapi) Platforms() []string { return []string{"linux"} }
func (vaapi) Supports(c Codec) bool {
	return c == CodecH264 || c == CodecHEVC || c == CodecAV1
}

func (vaapi) EncoderID(c Codec) string {
	switch c {
	case CodecH264:
		return "h264_vaapi"
	case CodecHEVC:
		return "hevc_vaapi"
	case CodecAV1:
		return "av1_vaapi"
	}
	return ""
}

// DeviceArgs initializes the VAAPI device; required for the encoder, so it is
// also applied during probing.
func (vaapi) DeviceArgs(s Spec) []string {
	dev := s.VAAPIDevice
	if dev == "" {
		dev = "/dev/dri/renderD128"
	}
	return []string{"-vaapi_device", dev}
}

// DecodeArgs is empty: we use software decode + hwupload (see OutputArgs) for
// robustness across drivers.
func (vaapi) DecodeArgs(s Spec) []string { return nil }

func (b vaapi) OutputArgs(s Spec) []string {
	// Any software scale/fps must run BEFORE the GPU upload (decode is software
	// here), so they prefix the existing format=nv12,hwupload chain. Then encode
	// with constant QP.
	chain := append(softwareFilters(s), "format=nv12", "hwupload")
	return []string{
		"-vf", strings.Join(chain, ","),
		"-c:v", b.EncoderID(s.Codec),
		"-qp", itoa(qpFromQuality(s.Quality)),
	}
}
