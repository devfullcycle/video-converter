package discover

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestFind(t *testing.T) {
	root := t.TempDir()
	// Layout:
	//   a.mp4, b.MOV (uppercase), c.txt (ignored), sub/d.mkv
	//   out/already.mp4  (output dir — must be skipped)
	mustWrite(t, filepath.Join(root, "a.mp4"))
	mustWrite(t, filepath.Join(root, "b.MOV"))
	mustWrite(t, filepath.Join(root, "c.txt"))
	mustWrite(t, filepath.Join(root, "sub", "d.mkv"))
	mustWrite(t, filepath.Join(root, "out", "already.mp4"))

	outDir := filepath.Join(root, "out")
	files, err := Find(root, outDir, []string{".mp4", ".mov", ".mkv"})
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}

	var rels []string
	for _, f := range files {
		rels = append(rels, filepath.ToSlash(f.Rel))
	}
	sort.Strings(rels)

	want := []string{"a.mp4", "b.MOV", "sub/d.mkv"}
	if len(rels) != len(want) {
		t.Fatalf("got %v, want %v", rels, want)
	}
	for i := range want {
		if rels[i] != want[i] {
			t.Errorf("rel[%d] = %q want %q", i, rels[i], want[i])
		}
	}
}

func TestFindEmpty(t *testing.T) {
	root := t.TempDir()
	files, err := Find(root, filepath.Join(root, "out"), []string{".mp4"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected no files, got %v", files)
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
