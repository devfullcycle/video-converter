package encoder

import (
	"math"
	"strconv"
)

// The single user-facing quality knob is 0..100 where higher means better
// visual quality. Each backend maps it onto its native control with a shared
// curve so that QUALITY=60 looks comparable across machines/encoders.

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// qpFromQuality maps 0..100 (higher better) to a 0..51 QP/CRF-style value
// (lower better). Used by libx264/libx265 (-crf), nvenc (-cq),
// qsv (-global_quality), amf (-qp_i) and vaapi (-qp). At Q=60 this yields ~29,
// at Q=80 ~22 (≈ x264 crf 22, visually high quality).
func qpFromQuality(q int) int {
	q = clampInt(q, 0, 100)
	return clampInt(int(math.Round(51-float64(q)*0.36)), 0, 51)
}

// av1CrfFromQuality maps 0..100 onto libsvtav1's wider 0..63 CRF range.
func av1CrfFromQuality(q int) int {
	q = clampInt(q, 0, 100)
	return clampInt(int(math.Round(63-float64(q)*0.44)), 0, 63)
}

// vtQualityFromQuality maps onto VideoToolbox's -q:v scale (1..100, higher
// better), i.e. it is essentially the identity with clamping.
func vtQualityFromQuality(q int) int {
	return clampInt(q, 1, 100)
}

// itoa is a tiny helper to keep arg builders readable.
func itoa(v int) string { return strconv.Itoa(v) }
