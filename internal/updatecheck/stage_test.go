package updatecheck

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testExeName() string {
	if runtime.GOOS == "windows" {
		return "pxelab.exe"
	}
	return "pxelab"
}

func makeZip(t *testing.T, path, name, content string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func makeTarGz(t *testing.T, path, name, content string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractBinaryZip(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "update.zip")
	makeZip(t, archive, "release/"+testExeName(), "zip-binary")

	out, err := extractBinary(archive, "1.2.3", dir)
	if err != nil {
		t.Fatalf("extractBinary() error = %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "zip-binary" {
		t.Fatalf("content = %q", data)
	}
	if !strings.Contains(filepath.Base(out), "1.2.3") {
		t.Fatalf("output name = %q, want version suffix", out)
	}
}

func TestExtractBinaryTarGz(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "update.tar.gz")
	makeTarGz(t, archive, testExeName(), "tar-binary")

	out, err := extractBinary(archive, "1.2.3", dir)
	if err != nil {
		t.Fatalf("extractBinary() error = %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "tar-binary" {
		t.Fatalf("content = %q", data)
	}
}

func TestExtractBinaryMissing(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "update.zip")
	makeZip(t, archive, "README.md", "readme")

	_, err := extractBinary(archive, "1.2.3", dir)
	if err == nil || !strings.Contains(err.Error(), "没有") {
		t.Fatalf("extractBinary() error = %v, want missing binary", err)
	}
}

func TestParseChecksums(t *testing.T) {
	input := "ABC123  pxelab_1.2.3_windows_amd64.zip\n" +
		"def456 *pxelab_1.2.3_linux_amd64.tar.gz\n" +
		"malformed line\n"
	sums, err := parseChecksums(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if sums["pxelab_1.2.3_windows_amd64.zip"] != "abc123" {
		t.Fatalf("zip checksum = %q", sums["pxelab_1.2.3_windows_amd64.zip"])
	}
	if sums["pxelab_1.2.3_linux_amd64.tar.gz"] != "def456" {
		t.Fatalf("tar checksum = %q", sums["pxelab_1.2.3_linux_amd64.tar.gz"])
	}
}
