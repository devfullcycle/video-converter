package convert

import (
	"fmt"
	"strings"
	"time"
)

// cappedBuffer is an io.Writer that retains only the last `limit` bytes written,
// so a chatty ffmpeg failure can't blow up memory.
type cappedBuffer struct {
	limit int
	buf   []byte
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	c.buf = append(c.buf, p...)
	if len(c.buf) > c.limit {
		c.buf = c.buf[len(c.buf)-c.limit:]
	}
	return len(p), nil
}

func (c *cappedBuffer) String() string { return string(c.buf) }

// lastLines returns the last n non-empty lines of s joined with "; ".
func lastLines(s string, n int) string {
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if t := strings.TrimSpace(l); t != "" {
			lines = append(lines, t)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "; ")
}

// printProgress emits a one-line status as each job completes.
func printProgress(res Result) {
	switch res.Status {
	case StatusOK:
		fmt.Printf("  ✓ %s  →  %s  (fim %s, levou %s)\n",
			res.Job.Rel, humanize(res.OutSize), res.FinishedAt.Format("15:04:05"), res.Elapsed.Round(time.Second))
	case StatusSkip:
		if res.Err != nil { // interrupted
			fmt.Printf("  • %s  (%v)\n", res.Job.Rel, res.Err)
		} else {
			fmt.Printf("  • %s  (já existe, pulado)\n", res.Job.Rel)
		}
	case StatusFail:
		fmt.Printf("  ✗ %s  —  %v\n", res.Job.Rel, res.Err)
	}
}

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
