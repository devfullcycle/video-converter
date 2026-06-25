package encoder

import "strings"

// This file centralizes the optional software video filters (downscale + frame
// rate cap) so every backend builds an identical, consistent filter chain.
//
// Both filters run in software (CPU), which works uniformly across all backends:
// none of the hardware-decode paths request -hwaccel_output_format, so ffmpeg
// downloads decoded frames to system memory and ordinary filters apply. The
// hardware *encoders* then re-upload as needed (VAAPI does so explicitly via
// hwupload; the others auto-insert it).

// scaleFilter caps the output height at s.MaxHeight while preserving the aspect
// ratio and keeping the width even (-2). It never upscales: min(H,ih) leaves
// shorter sources untouched. Returns "" when no scaling is requested.
//
// The comma inside min() is escaped (\,) so it isn't read as a filter separator
// when chained; ffmpeg unescapes it back to min(H,ih) at parse time.
func scaleFilter(s Spec) string {
	if s.MaxHeight <= 0 {
		return ""
	}
	return "scale=-2:min(" + itoa(s.MaxHeight) + "\\,ih)"
}

// fpsFilter caps the frame rate at s.FPS frames per second; "" when disabled.
func fpsFilter(s Spec) string {
	if s.FPS <= 0 {
		return ""
	}
	return "fps=" + itoa(s.FPS)
}

// softwareFilters returns the ordered list of requested software filters
// (scale before fps), or nil when neither is requested.
func softwareFilters(s Spec) []string {
	var f []string
	if v := scaleFilter(s); v != "" {
		f = append(f, v)
	}
	if v := fpsFilter(s); v != "" {
		f = append(f, v)
	}
	return f
}

// vfArgs returns the standalone "-vf <chain>" argument pair for backends that
// have no other filters, or nil when no software filters are requested. Backends
// with their own filter chain (e.g. VAAPI's hwupload) call softwareFilters
// directly and merge instead.
func vfArgs(s Spec) []string {
	f := softwareFilters(s)
	if len(f) == 0 {
		return nil
	}
	return []string{"-vf", strings.Join(f, ",")}
}
