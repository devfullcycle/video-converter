package encoder

// cpu is the software-encoding backend (libx264/libx265/libsvtav1). It is the
// guaranteed fallback and works on every platform.
type cpu struct{}

func (cpu) Name() Backend         { return BackendCPU }
func (cpu) Platforms() []string   { return []string{"linux", "windows", "darwin"} }
func (cpu) Supports(c Codec) bool { return c == CodecH264 || c == CodecHEVC || c == CodecAV1 }

func (cpu) EncoderID(c Codec) string {
	switch c {
	case CodecH264:
		return "libx264"
	case CodecHEVC:
		return "libx265"
	case CodecAV1:
		return "libsvtav1"
	}
	return ""
}

func (cpu) DeviceArgs(s Spec) []string { return nil }
func (cpu) DecodeArgs(s Spec) []string { return nil }

func (b cpu) OutputArgs(s Spec) []string {
	args := vfArgs(s)
	args = append(args, "-c:v", b.EncoderID(s.Codec))
	if s.Codec == CodecAV1 {
		args = append(args, "-crf", itoa(av1CrfFromQuality(s.Quality)), "-preset", svtAV1Preset(s.Intensity))
	} else {
		args = append(args, "-crf", itoa(qpFromQuality(s.Quality)), "-preset", x26xPreset(s.Intensity))
	}
	if s.Threads > 0 {
		args = append(args, "-threads", itoa(s.Threads))
	}
	return args
}

// x26xPreset maps intensity onto libx264/libx265 presets. Higher intensity =
// slower preset = better compression, but more CPU.
func x26xPreset(i Intensity) string {
	switch i {
	case IntensityLight:
		return "veryfast"
	case IntensityMax:
		return "slow"
	default:
		return "medium"
	}
}

// svtAV1Preset maps intensity onto libsvtav1's numeric preset (0 slowest/best,
// 13 fastest). We stay in a sane range for a general-purpose converter.
func svtAV1Preset(i Intensity) string {
	switch i {
	case IntensityLight:
		return "10"
	case IntensityMax:
		return "6"
	default:
		return "8"
	}
}
