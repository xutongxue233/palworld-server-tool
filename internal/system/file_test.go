package system

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func createZip(t *testing.T, path, entryName string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create(entryName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestUnzipDirRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bad.zip")
	destination := filepath.Join(root, "destination")
	createZip(t, archivePath, "../outside.txt")

	if err := UnzipDir(archivePath, destination); err == nil {
		t.Fatal("expected traversal archive to be rejected")
	}
	if _, err := os.Stat(filepath.Join(root, "outside.txt")); !os.IsNotExist(err) {
		t.Fatalf("archive wrote outside destination: %v", err)
	}
}

func TestUnzipDirExtractsSafeEntry(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "safe.zip")
	destination := filepath.Join(root, "destination")
	createZip(t, archivePath, "nested/Level.sav")

	if err := UnzipDir(archivePath, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "nested", "Level.sav")); err != nil {
		t.Fatal(err)
	}
}

func TestUnTarGzDirRejectsPathTraversal(t *testing.T) {
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "../outside.txt",
		Mode: 0600,
		Size: 4,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	destination := filepath.Join(root, "destination")
	if err := UnTarGzDir(bytes.NewReader(buffer.Bytes()), destination); err == nil {
		t.Fatal("expected traversal archive to be rejected")
	}
	if _, err := os.Stat(filepath.Join(root, "outside.txt")); !os.IsNotExist(err) {
		t.Fatalf("archive wrote outside destination: %v", err)
	}
}

func TestCopyAndZipRejectSymbolicLinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(source, "linked.sav")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symbolic links are not available in this environment: %v", err)
	}

	if err := CopyFile(link, filepath.Join(root, "copied.sav")); err == nil {
		t.Fatal("CopyFile followed a symbolic link")
	}
	if err := CopyDir(source, filepath.Join(root, "copy")); err == nil {
		t.Fatal("CopyDir followed a symbolic link")
	}
	if err := ZipDir(source, filepath.Join(root, "archive.zip")); err == nil {
		t.Fatal("ZipDir followed a symbolic link")
	}
	if _, err := os.Stat(filepath.Join(root, "archive.zip")); !os.IsNotExist(err) {
		t.Fatalf("failed archive was not removed: %v", err)
	}
}
