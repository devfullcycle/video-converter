// Package report summarizes a batch of conversions.
package report

import (
	"fmt"
	"io"
	"time"

	"github.com/devfullcycle/video-converter/internal/convert"
)

// Summary aggregates the results of a run.
type Summary struct {
	OK, Skipped, Failed int
	TotalIn, TotalOut   int64
	Wall                time.Duration
}

// Summarize folds results into a Summary.
func Summarize(results []convert.Result, wall time.Duration) Summary {
	var s Summary
	s.Wall = wall
	for _, r := range results {
		switch r.Status {
		case convert.StatusOK:
			s.OK++
			s.TotalIn += r.InSize
			s.TotalOut += r.OutSize
		case convert.StatusSkip:
			s.Skipped++
		case convert.StatusFail:
			s.Failed++
		}
	}
	return s
}

// Print writes a human-readable summary.
func Print(w io.Writer, s Summary) {
	fmt.Fprintf(w, "\nResumo: %d convertidos, %d pulados, %d falharam — tempo total %s\n",
		s.OK, s.Skipped, s.Failed, s.Wall.Round(time.Second))
	if s.TotalIn > 0 && s.TotalOut > 0 {
		reduction := 100 * (1 - float64(s.TotalOut)/float64(s.TotalIn))
		fmt.Fprintf(w, "Tamanho: %s → %s (%.1f%% menor)\n",
			humanize(s.TotalIn), humanize(s.TotalOut), reduction)
	}
}

// HasFailures reports whether any job failed (for the process exit code).
func (s Summary) HasFailures() bool { return s.Failed > 0 }

func humanize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
