// Command video-converter batch-converts videos with ffmpeg, automatically
// choosing the best available hardware encoder (NVENC/QSV/AMF/VAAPI/
// VideoToolbox) or CPU, configurable via flags or a .env file.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/devfullcycle/video-converter/internal/config"
	"github.com/devfullcycle/video-converter/internal/convert"
	"github.com/devfullcycle/video-converter/internal/discover"
	"github.com/devfullcycle/video-converter/internal/encoder"
	"github.com/devfullcycle/video-converter/internal/report"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load(".env", os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro de configuração:", err)
		return 2
	}

	// Graceful shutdown: Ctrl-C cancels the context, which kills in-flight
	// ffmpeg processes and lets partial files be cleaned up.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	env, err := encoder.DetectEnvironment(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 2
	}

	sel, err := encoder.Select(ctx, env, cfg.Backend, cfg.Codec, cfg.FallbackOnFailure, cfg.VAAPIDevice)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nenhum encoder utilizável:", err)
		return 2
	}

	files, err := discover.Find(cfg.InputDir, cfg.OutputDir, cfg.Extensions)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro ao varrer entrada:", err)
		return 2
	}
	if len(files) == 0 {
		fmt.Printf("Nenhum arquivo (%v) encontrado em %s\n", cfg.Extensions, cfg.InputDir)
		return 0
	}

	outputBase := filepath.Join(cfg.OutputDir, filepath.Base(filepath.Clean(cfg.InputDir))+"_CONV")
	jobs := convert.PlanJobs(files, outputBase, cfg.Codec)

	workers := cfg.Workers
	if workers == 0 {
		workers = defaultWorkers(sel.Backend, cfg.Intensity)
	}
	threads := cfg.Threads
	if threads == 0 && sel.Backend == encoder.BackendCPU {
		threads = defaultThreads(workers)
	}
	logDir := cfg.LogDir
	if logDir == "" {
		logDir = "logs"
	}

	printHeader(cfg, sel, len(jobs), workers)

	start := time.Now()
	results, _ := convert.Run(ctx, jobs, convert.Options{
		FFmpeg:      env.FFmpegPath,
		Backend:     sel.Backend,
		Codec:       cfg.Codec,
		Quality:     cfg.Quality,
		FPS:         cfg.FPS,
		MaxHeight:   cfg.MaxHeight,
		Intensity:   cfg.Intensity,
		Threads:     threads,
		VAAPIDevice: cfg.VAAPIDevice,
		Workers:     workers,
		Overwrite:   cfg.Overwrite,
		Nice:        cfg.Nice,
		LogDir:      logDir,
	})

	summary := report.Summarize(results, time.Since(start))
	report.Print(os.Stdout, summary)

	if ctx.Err() != nil {
		fmt.Fprintln(os.Stderr, "\ninterrompido pelo usuário.")
		return 130
	}
	if summary.HasFailures() {
		fmt.Fprintf(os.Stderr, "logs de erro em: %s\n", logDir)
		return 1
	}
	return 0
}

func printHeader(cfg *config.Config, sel *encoder.Selection, nJobs, workers int) {
	fmt.Println("Detecção de encoder:")
	for _, line := range sel.Trace {
		fmt.Printf("  - %s\n", line)
	}
	fmt.Printf("\nBackend: %s (%s)  |  Codec: %s  |  Qualidade: %d  |  Intensidade: %s  |  Workers: %d\n",
		sel.Backend, sel.EncoderID, cfg.Codec, cfg.Quality, cfg.Intensity, workers)
	fmt.Printf("Frame rate: %s  |  Escala: %s  |  Áudio: copiado (sem reencode)\n",
		fpsLabel(cfg.FPS), scaleLabel(cfg.MaxHeight))
	fmt.Printf("Convertendo %d arquivo(s) de %s\n\n", nJobs, cfg.InputDir)
}

// fpsLabel renders the frame-rate setting for the header (0 = unchanged).
func fpsLabel(fps int) string {
	if fps <= 0 {
		return "original"
	}
	return fmt.Sprintf("%d fps", fps)
}

// scaleLabel renders the scaling setting for the header (0 = unchanged).
func scaleLabel(h int) string {
	if h <= 0 {
		return "original"
	}
	return fmt.Sprintf("até %dp", h)
}

// defaultWorkers picks a sensible concurrency per backend. Hardware encoders
// share a single engine, so more workers don't help (and can hit session
// limits); CPU scales with cores, scaled by the intensity knob.
func defaultWorkers(b encoder.Backend, intensity encoder.Intensity) int {
	switch b {
	case encoder.BackendCPU:
		n := runtime.NumCPU()
		switch intensity {
		case encoder.IntensityLight:
			return 1
		case encoder.IntensityMax:
			return max(1, n-1)
		default:
			return max(1, n/2)
		}
	case encoder.BackendVAAPI:
		return 1
	default: // nvenc, qsv, amf, videotoolbox
		return 2
	}
}

// defaultThreads caps ffmpeg's CPU threads so workers*threads stays near the
// core count and the machine remains usable.
func defaultThreads(workers int) int {
	if workers <= 0 {
		return 0
	}
	return max(1, runtime.NumCPU()/workers)
}
