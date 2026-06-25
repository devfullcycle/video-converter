# CLAUDE.md

Guidance for AI agents (and humans) working in this repository.

## Overview

`video-converter` is a Go CLI that batch-converts videos with `ffmpeg`. It auto-detects the best
available encoder (NVENC / QSV / AMF / VAAPI / VideoToolbox) or falls back to CPU (libx264/x265/SVT-AV1),
and is configurable per machine via flags or a `.env` file. The user-facing docs are in `README.md`
(pt-BR) and `.env.example`. Code and identifiers are in English.

## Commands

```bash
go build -o video-converter .   # build the CLI
go test ./...                   # unit + integration tests (integration needs ffmpeg in PATH)
go test -short ./...            # unit tests only (no ffmpeg required)
go vet ./...                    # static checks
gofmt -l .                      # list unformatted files (must be empty; CI fails otherwise)
./video-converter -input ./in -output ./out   # run (or rely on a .env in the CWD)
```

CI (`.github/workflows/ci.yml`) runs gofmt, vet, `go test -race`, and build on Go 1.22.

## Architecture

Thin `main.go` wires the pipeline: **config → detect/select → discover → run → report**.

| Package | Responsibility |
|---|---|
| `internal/config` | `Config`, flag parsing, `.env` loader, precedence resolution |
| `internal/encoder` | **Core**: backend interface, `BuildArgs`, per-encoder recipes, detection, functional probe, selection |
| `internal/discover` | Recursive file discovery (`WalkDir`, case-insensitive ext match, skips output dir) |
| `internal/convert` | Job planning + the concurrent runner (errgroup + context + temp-file + stderr capture) |
| `internal/report` | Summary (counts, size reduction, exit code) |

## Core abstraction (`internal/encoder`)

Each backend implements `EncoderBackend` (see `backend.go`). The pure function `BuildArgs(Spec)` in
`profile.go` assembles the ffmpeg argument slice in this **fixed order**:

```
-hide_banner -loglevel error -nostdin -y  <DeviceArgs> <DecodeArgs> -i <input>  <OutputArgs>  -c:a copy  [container]  <output>
```

- **`DeviceArgs`** = encoder init that precedes `-i` and is needed even when probing (e.g. `-vaapi_device`).
- **`DecodeArgs`** = hardware *decode* of the real input (e.g. `-hwaccel cuda`); real runs only — **probes must not apply it** (the probe source is synthetic software frames).
- **`OutputArgs`** = optional software filters (`-vf`) + codec + the single 0–100 quality knob mapped to the encoder's native control (`-crf`/`-cq`/`-global_quality`/`-qp`/`-q:v`) + preset/tune.

The optional video filters (frame-rate cap + downscale) live in `filters.go` and are built once,
identically for every backend: `scale=-2:min(H\,ih)` (never upscales, keeps width even) and `fps=N`.
Most backends prepend `vfArgs(s)`; VAAPI merges `softwareFilters(s)` ahead of its `format=nv12,hwupload`
chain (decode is software there). They run only when `Spec.FPS`/`Spec.MaxHeight` are > 0, so the probe
(which leaves them 0) and runs with `FPS=0`/`SCALE=0` add no `-vf`. The filters are software because no
backend sets `-hwaccel_output_format`, so decoded frames are in system memory.

Backend selection (`select.go`) filters by `runtime.GOOS` and presence in `ffmpeg -encoders`, then runs a
**functional probe** (`probe.go`): a real 1-frame encode of a synthetic source. Presence is necessary but
not sufficient — the probe is what makes `auto` reliably skip a non-working GPU and land on CPU.

### Adding a new backend

1. Add a file implementing `EncoderBackend` (model it on `nvenc.go` / `vaapi.go`).
2. Register it in `registry()` in `backend.go` (order = auto-detect priority; CPU stays last).
3. Add a table-driven case to `profile_test.go`.

## Conventions & invariants (don't break these)

- **`BuildArgs` is pure** (no ffmpeg, no filesystem). Keep it that way; it's the main test target.
- **Atomic output**: the runner encodes to `<name>.part.<ext>` and renames on success (cross-FS copy
  fallback). A present output file is always complete; partials are removed on failure/cancel.
- **Never hang**: always pass `-nostdin -y`; skip-existing is enforced on the final path in the runner.
- **Per-file failures never abort the batch**: `g.Go` callbacks always return `nil`; only the parent
  context (Ctrl-C via `signal.NotifyContext` + `exec.CommandContext`) cancels the run.
- **errgroup gotcha**: `errgroup.WithContext` cancels its derived context when `Wait` returns. Use the
  derived `gctx` to launch ffmpeg, but inspect the **parent** `ctx.Err()` for interruption (see `runner.go`).
- **Worker defaults differ by backend** (`main.go: defaultWorkers`): hardware = 1–2 (one encode engine),
  CPU scales with cores via `INTENSITY`. More workers do not speed up GPU.
- **Quality knob is 0–100, higher = better**; mappings live in `quality.go` and must stay monotonic.

## Gotchas

- **Probe resolution is 320x240** on purpose — NVENC rejects frames below its minimum size; smaller
  sources cause false-negative probes.
- **`OUTPUT_DIR` must not equal `INPUT_DIR`**. Discovery skips the output dir; if it equals the input
  root, the walk skips everything. Point `OUTPUT_DIR` at a different/parent folder.
- **`x/sync` is pinned to v0.7.0** to keep the `go 1.22` directive (newer versions bump the required Go).
- **Audio is `-c:a copy`** (fast, lossless). Rare incompatible source audio in MP4 could fail a job; the
  stderr log will show it.
- **VideoToolbox** supports H.264/HEVC only (no AV1) and constant quality only on Apple Silicon; we pass
  `-allow_sw 1` so it degrades gracefully.
- **`fps=N` can duplicate frames** for sub-N sources (the filter has no "min with source rate"); harmless
  and the encoder dedups well. `scale` does guard against upscaling via `min(H,ih)`.

## Notes

- `.env` is loaded from the **current working directory** (not the binary's dir) and never clobbers
  variables already set in the real environment. `.env` is gitignored; `.env.example` is the template.