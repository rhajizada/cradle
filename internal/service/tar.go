package service

import (
	"archive/tar"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func TarDir(dir string) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(writeTarDir(dir, pw))
	}()

	return pr
}

func writeTarDir(dir string, pw *io.PipeWriter) error {
	tw := tar.NewWriter(pw)
	defer tw.Close()

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return writeEntry(dir, path, d, tw)
	})
}

func writeEntry(baseDir, path string, d fs.DirEntry, tw *tar.Writer) error {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	rel = filepath.ToSlash(rel)

	info, err := d.Info()
	if err != nil {
		return err
	}

	link, err := symlinkTarget(info, path)
	if err != nil {
		return err
	}

	hdr, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	hdr.Name = normalizeDirName(rel, info)

	if headerErr := tw.WriteHeader(hdr); headerErr != nil {
		return headerErr
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	return copyFileToTar(path, tw)
}

func symlinkTarget(info os.FileInfo, path string) (string, error) {
	if info.Mode()&os.ModeSymlink == 0 {
		return "", nil
	}
	return os.Readlink(path)
}

func normalizeDirName(name string, info os.FileInfo) string {
	if info.IsDir() && !strings.HasSuffix(name, "/") {
		return name + "/"
	}
	return name
}

func copyFileToTar(path string, tw *tar.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(tw, f)
	return err
}
