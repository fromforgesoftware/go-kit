package helpers

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func getGoldenFile(t *testing.T, filePath string, flg int, mode fs.FileMode) *os.File {
	t.Helper()

	goldenPath := filePath
	if filepath.Ext(filePath) != ".golden" {
		goldenPath = filePath + ".golden"
	}

	//nolint:gosec //file path is used for testing purposes, so it's a controlled env
	f, err := os.OpenFile(goldenPath, flg, mode)
	assert.NoError(t, err)
	assert.NotNil(t, f)

	t.Cleanup(
		func() {
			assert.NoError(t, f.Close())
		},
	)

	return f
}

func updateFile(t *testing.T, file *os.File, bs []byte) {
	t.Helper()

	_, err := file.Write(bs)
	assert.NoError(t, err)
}

func fileStartSeek(t *testing.T, f *os.File) {
	t.Helper()

	_, err := f.Seek(0, 0)
	assert.NoError(t, err)
}

func AssertEqualFile(t *testing.T, filePath string, content io.Reader, updateGoldenFile bool) {
	t.Helper()

	got, err := io.ReadAll(content)
	assert.NoError(t, err)

	//nolint:mnd // not a magic constant
	flags := os.O_RDONLY
	mode := os.FileMode(0o644)
	if updateGoldenFile {
		flags = os.O_CREATE | os.O_RDWR | os.O_TRUNC
	}
	f := getGoldenFile(t, filePath, flags, mode)
	if updateGoldenFile {
		t.Logf("updating golden file: %s", filePath)
		updateFile(t, f, got)
		fileStartSeek(t, f)
		t.Logf("golden file: %s updated", filePath)
	}

	goldenBs, err := io.ReadAll(f)
	assert.NoError(t, err)
	assert.Equalf(t,
		goldenBs, got,
		"--want--\n%s\n--got--\n%s",
		goldenBs, got,
	)
}

func AssertEqualGoldenFile(t *testing.T, filePath string, content io.Reader, updateGoldenFile bool) {
	t.Helper()

	got, err := io.ReadAll(content)
	assert.NoError(t, err)

	//nolint:mnd // not a magic constant
	flags := os.O_RDONLY
	mode := os.FileMode(0o644)
	if updateGoldenFile {
		flags = os.O_CREATE | os.O_RDWR | os.O_TRUNC
	}
	f := getGoldenFile(t, filePath, flags, mode)
	if updateGoldenFile {
		t.Logf("updating golden file: %s", filePath)
		updateFile(t, f, got)
		fileStartSeek(t, f)
		t.Logf("golden file: %s updated", filePath)
	}

	goldenBs, err := io.ReadAll(f)
	assert.NoError(t, err)
	assert.Equalf(t,
		goldenBs, got,
		"--want--\n%s\n--got--\n%s",
		goldenBs, got,
	)
}
