// Package discover finds the video files to convert under an input directory.
package discover

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// File is a discovered input file and its path relative to the input root.
type File struct {
	Path string
	Rel  string
}

// Find walks inputDir (recursively) and returns every file whose extension is
// in exts (case-insensitive). The output directory subtree is skipped so a
// re-run never tries to re-convert our own output.
func Find(inputDir, outputDir string, exts []string) ([]File, error) {
	extSet := make(map[string]bool, len(exts))
	for _, e := range exts {
		extSet[strings.ToLower(e)] = true
	}

	absOut, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, err
	}

	var files []File
	walkErr := filepath.WalkDir(inputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if abs, _ := filepath.Abs(path); abs == absOut {
				return filepath.SkipDir
			}
			return nil
		}
		if extSet[strings.ToLower(filepath.Ext(path))] {
			rel, err := filepath.Rel(inputDir, path)
			if err != nil {
				return err
			}
			files = append(files, File{Path: path, Rel: rel})
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return files, nil
}
