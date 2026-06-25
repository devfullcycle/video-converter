package encoder

import (
	"context"
	"fmt"
)

// Selection is the outcome of choosing a backend, plus a human-readable trace
// of what was tried (printed once at startup).
type Selection struct {
	Backend   Backend
	EncoderID string
	Trace     []string
}

// Select resolves the requested backend (or "auto") into a concrete, verified
// backend for the given codec.
//
//   - requested == auto: try each backend in priority order (strongest hardware
//     first), functionally probing each, and select the first that works. CPU is
//     always last and acts as the guaranteed fallback.
//   - requested == a specific backend: probe it; on success use it. On failure,
//     fall back to CPU when fallback is true (with a loud trace), otherwise
//     return an error so a demo/CI run fails fast instead of silently using the
//     "wrong" engine.
func Select(ctx context.Context, env *Environment, requested Backend, codec Codec, fallback bool, vaapiDevice string) (*Selection, error) {
	var trace []string

	attempt := func(be EncoderBackend) (bool, string) {
		if !supportsOS(be.Platforms(), CurrentOS) {
			return false, fmt.Sprintf("not available on %s", CurrentOS)
		}
		if !be.Supports(codec) {
			return false, fmt.Sprintf("does not support codec %s", codec)
		}
		id := be.EncoderID(codec)
		if !env.Encoders[id] {
			return false, fmt.Sprintf("encoder %s not built into this ffmpeg", id)
		}
		if err := env.Prober(ctx, be, codec, vaapiDevice); err != nil {
			return false, fmt.Sprintf("probe failed (%v)", err)
		}
		return true, ""
	}

	if requested == "" || requested == BackendAuto {
		for _, be := range registry() {
			ok, reason := attempt(be)
			if ok {
				trace = append(trace, fmt.Sprintf("%s: OK — selected", be.Name()))
				return &Selection{Backend: be.Name(), EncoderID: be.EncoderID(codec), Trace: trace}, nil
			}
			trace = append(trace, fmt.Sprintf("%s: %s", be.Name(), reason))
		}
		return nil, fmt.Errorf("no working encoder found for codec %s:\n  %s", codec, joinTrace(trace))
	}

	// Explicit backend request.
	be, ok := Lookup(requested)
	if !ok {
		return nil, fmt.Errorf("unknown backend %q (valid: %v)", requested, AllBackends())
	}
	if ok, reason := attempt(be); ok {
		trace = append(trace, fmt.Sprintf("%s: OK — selected", be.Name()))
		return &Selection{Backend: be.Name(), EncoderID: be.EncoderID(codec), Trace: trace}, nil
	} else {
		trace = append(trace, fmt.Sprintf("%s: %s", be.Name(), reason))
		if !fallback {
			return nil, fmt.Errorf("backend %q unavailable and fallback is disabled: %s", requested, reason)
		}
	}

	// Fall back to CPU.
	cpuBE, _ := Lookup(BackendCPU)
	if ok, reason := attempt(cpuBE); ok {
		trace = append(trace, "cpu: OK — selected (fallback)")
		return &Selection{Backend: BackendCPU, EncoderID: cpuBE.EncoderID(codec), Trace: trace}, nil
	} else {
		trace = append(trace, fmt.Sprintf("cpu: %s", reason))
	}
	return nil, fmt.Errorf("requested backend %q failed and CPU fallback also failed:\n  %s", requested, joinTrace(trace))
}

func joinTrace(trace []string) string {
	out := ""
	for i, t := range trace {
		if i > 0 {
			out += "\n  "
		}
		out += t
	}
	return out
}
