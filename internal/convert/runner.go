package convert

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/devfullcycle/video-converter/internal/encoder"
	"golang.org/x/sync/errgroup"
)

// Status is the outcome of a single job.
type Status string

const (
	StatusOK   Status = "ok"
	StatusSkip Status = "skip"
	StatusFail Status = "fail"
)

// Result is the outcome of converting one file.
type Result struct {
	Job        Job
	Status     Status
	Err        error
	InSize     int64
	OutSize    int64
	StartedAt  time.Time
	FinishedAt time.Time
	Elapsed    time.Duration
}

// Options configures a Run.
type Options struct {
	FFmpeg      string
	Backend     encoder.Backend
	Codec       encoder.Codec
	Quality     int
	FPS         int
	MaxHeight   int
	Intensity   encoder.Intensity
	Threads     int
	VAAPIDevice string
	Workers     int
	Overwrite   bool
	Nice        int
	LogDir      string
}

// Run converts all jobs with at most opts.Workers concurrent ffmpeg processes.
//
// A per-file failure never aborts the batch: it is captured in that file's
// Result and the run continues. The errgroup is therefore only cancelled by the
// caller's context (e.g. Ctrl-C), which kills in-flight ffmpeg processes via
// exec.CommandContext. Results are returned in input order.
func Run(ctx context.Context, jobs []Job, opts Options) ([]Result, error) {
	results := make([]Result, len(jobs))

	// gctx is the errgroup-derived context used to launch/kill ffmpeg. The
	// parent ctx is what we inspect afterwards for interruption, because
	// errgroup cancels gctx itself once Wait returns.
	g, gctx := errgroup.WithContext(ctx)
	if opts.Workers > 0 {
		g.SetLimit(opts.Workers)
	}

	var mu sync.Mutex // guards stdout progress lines

	// onStart prints the start clock time the moment a file's conversion begins.
	onStart := func(rel string, at time.Time) {
		mu.Lock()
		fmt.Printf("  ▶ %s  (início %s)\n", rel, at.Format("15:04:05"))
		mu.Unlock()
	}

	for i, j := range jobs {
		i, j := i, j
		g.Go(func() error {
			if gctx.Err() != nil {
				results[i] = Result{Job: j, Status: StatusSkip, Err: gctx.Err()}
				return nil
			}
			res := runOne(gctx, opts, j, onStart)
			mu.Lock()
			printProgress(res)
			mu.Unlock()
			results[i] = res
			return nil // never propagate per-file errors to errgroup
		})
	}

	_ = g.Wait()
	return results, ctx.Err()
}

func runOne(ctx context.Context, opts Options, j Job, onStart func(string, time.Time)) Result {
	res := Result{Job: j, Status: StatusFail}

	if fi, err := os.Stat(j.Input); err == nil {
		res.InSize = fi.Size()
	}

	// Skip if a finished output already exists (unless overwriting).
	if !opts.Overwrite {
		if _, err := os.Stat(j.Output); err == nil {
			res.Status = StatusSkip
			return res
		}
	}

	if err := os.MkdirAll(filepath.Dir(j.Output), 0o755); err != nil {
		res.Err = fmt.Errorf("creating output dir: %w", err)
		return res
	}

	spec := encoder.Spec{
		Backend:     opts.Backend,
		Codec:       opts.Codec,
		Quality:     opts.Quality,
		FPS:         opts.FPS,
		MaxHeight:   opts.MaxHeight,
		Intensity:   opts.Intensity,
		Threads:     opts.Threads,
		InputPath:   j.Input,
		OutputPath:  j.Temp,
		VAAPIDevice: opts.VAAPIDevice,
	}
	args, err := encoder.BuildArgs(spec)
	if err != nil {
		res.Err = err
		return res
	}

	name, fullArgs := niceWrap(opts.FFmpeg, args, opts.Nice)
	cmd := exec.CommandContext(ctx, name, fullArgs...)
	stderr := &cappedBuffer{limit: 64 << 10}
	cmd.Stderr = stderr

	started := time.Now()
	res.StartedAt = started
	if onStart != nil {
		onStart(j.Rel, started)
	}

	runErr := cmd.Run()
	if runErr != nil {
		// Remove the partial temp file so a present output is always complete.
		os.Remove(j.Temp)
		writeLog(opts.LogDir, j, stderr.String())
		if ctx.Err() != nil {
			res.Status = StatusSkip
			res.Err = fmt.Errorf("interrupted")
			return res
		}
		res.Err = fmt.Errorf("ffmpeg: %v: %s", runErr, lastLines(stderr.String(), 3))
		return res
	}

	// Atomically promote the temp file to the final path.
	if err := os.Rename(j.Temp, j.Output); err != nil {
		// Cross-filesystem rename fallback: copy then remove.
		if cerr := copyFile(j.Temp, j.Output); cerr != nil {
			os.Remove(j.Temp)
			res.Err = fmt.Errorf("promoting temp file: %w", cerr)
			return res
		}
		os.Remove(j.Temp)
	}

	if fi, err := os.Stat(j.Output); err == nil {
		res.OutSize = fi.Size()
	}
	res.Status = StatusOK
	res.FinishedAt = time.Now()
	res.Elapsed = res.FinishedAt.Sub(started)
	return res
}

// niceWrap wraps the command with `nice -n N` on Unix when a positive niceness
// is requested and the `nice` binary is available; otherwise it's a no-op.
func niceWrap(ff string, args []string, nice int) (string, []string) {
	if nice <= 0 || runtime.GOOS == "windows" {
		return ff, args
	}
	if p, err := exec.LookPath("nice"); err == nil {
		return p, append([]string{"-n", strconv.Itoa(nice), ff}, args...)
	}
	return ff, args
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}

func writeLog(logDir string, j Job, content string) {
	if logDir == "" || content == "" {
		return
	}
	logPath := filepath.Join(logDir, j.Rel+".log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(logPath, []byte(content), 0o644)
}
