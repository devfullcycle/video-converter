// Package convert plans and executes the per-file ffmpeg conversions.
package convert

import (
	"path/filepath"
	"strings"

	"github.com/devfullcycle/video-converter/internal/discover"
	"github.com/devfullcycle/video-converter/internal/encoder"
)

// Job describes one conversion: the source, the final destination, and the
// temporary path ffmpeg writes to before the atomic rename.
type Job struct {
	Input  string
	Output string // final path
	Temp   string // ffmpeg writes here, then we rename to Output on success
	Rel    string // path relative to input root (for logs/reporting)
}

// PlanJobs maps discovered files to jobs, mirroring the input tree under
// outputBase and normalizing the output extension to the codec's container.
func PlanJobs(files []discover.File, outputBase string, codec encoder.Codec) []Job {
	ext := encoder.ContainerForCodec(codec)
	jobs := make([]Job, 0, len(files))
	for _, f := range files {
		relOut := replaceExt(f.Rel, ext)
		out := filepath.Join(outputBase, relOut)
		jobs = append(jobs, Job{
			Input:  f.Path,
			Output: out,
			Temp:   tempPath(out, ext),
			Rel:    relOut,
		})
	}
	return jobs
}

// replaceExt swaps a path's extension for newExt (which includes the dot).
func replaceExt(p, newExt string) string {
	return strings.TrimSuffix(p, filepath.Ext(p)) + newExt
}

// tempPath inserts ".part" before the extension so ffmpeg can still infer the
// muxer from the extension (e.g. out.mp4 -> out.part.mp4).
func tempPath(finalPath, ext string) string {
	return strings.TrimSuffix(finalPath, ext) + ".part" + ext
}
