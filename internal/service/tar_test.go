package service

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestTarDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	rc, err := tarDir(dir)
	if err != nil {
		t.Fatalf("tarDir error: %v", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	tr := tar.NewReader(rc)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read error: %v", err)
		}
		if hdr.Name == "file.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected file in tar")
	}
}
