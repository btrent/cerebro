package images

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsImagePath(t *testing.T) {
	yes := []string{"a.png", "/x/y.JPG", "pic.jpeg", "anim.gif", "p.webp"}
	no := []string{"a.txt", "note.md", "noext", "archive.zip"}
	for _, s := range yes {
		if !IsImagePath(s) {
			t.Errorf("expected %q to be an image", s)
		}
	}
	for _, s := range no {
		if IsImagePath(s) {
			t.Errorf("expected %q to NOT be an image", s)
		}
	}
}

func TestDetectPaths(t *testing.T) {
	// Detection is whitespace-token based (paths with spaces are not supported).
	in := `look at ./a.png and /img/b.jpg plus ./a.png again and notes.txt`
	got := DetectPaths(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 unique image paths, got %v", got)
	}
	if got[0] != "./a.png" || got[1] != "/img/b.jpg" {
		t.Errorf("unexpected detected paths: %v", got)
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load("/nonexistent/zzz.png"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadUnsupported(t *testing.T) {
	if _, err := Load("/tmp/whatever.txt"); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestLoadOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	if err := os.WriteFile(path, []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}
	img, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if img.MIME != "image/png" || len(img.Data) != 4 {
		t.Errorf("unexpected image: %+v", img)
	}
}
