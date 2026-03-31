package testutil

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/linkdata/jaws/staticserve"
)

// ExpectedStaticAsset describes one expected static asset used in tests.
type ExpectedStaticAsset struct {
	Filepath    string
	Name        string
	URI         string
	ContentType string
	Plain       []byte
	Gz          []byte
	SS          *staticserve.StaticServe
}

// ReadGzip unpacks gzipped bytes and fails the test on error.
func ReadGzip(t *testing.T, b []byte) []byte {
	t.Helper()
	gzr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(gzr)
	if cerr := gzr.Close(); cerr != nil && err == nil {
		err = cerr
	}
	if err != nil {
		t.Fatal(err)
	}
	return plain
}

// AssetFilepaths returns sorted, relative asset paths under root.
func AssetFilepaths(t *testing.T, fsys fs.FS, root string) (filepaths []string) {
	t.Helper()
	root = path.Clean(root)
	err := fs.WalkDir(fsys, root, func(pathname string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		filepaths = append(filepaths, strings.TrimPrefix(pathname, root+"/"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(filepaths)
	if len(filepaths) == 0 {
		t.Fatal("expected at least one asset file")
	}
	return
}

// ExpectedStaticAssets builds expected static asset metadata for tests.
func ExpectedStaticAssets(t *testing.T, fsys fs.FS, root, uriPrefix string, filepaths ...string) (expected []ExpectedStaticAsset) {
	t.Helper()
	if len(filepaths) == 0 {
		filepaths = AssetFilepaths(t, fsys, root)
	}
	for _, filepath := range filepaths {
		b, err := fs.ReadFile(fsys, path.Join(root, filepath))
		if err != nil {
			t.Fatal(err)
		}
		ss, err := staticserve.New(filepath, b)
		if err != nil {
			t.Fatal(err)
		}
		plain := b
		if strings.HasSuffix(filepath, ".gz") {
			plain = ReadGzip(t, b)
		}
		expected = append(expected, ExpectedStaticAsset{
			Filepath:    filepath,
			Name:        ss.Name,
			URI:         path.Join(uriPrefix, ss.Name),
			ContentType: ss.ContentType,
			Plain:       plain,
			Gz:          ss.Gz,
			SS:          ss,
		})
	}
	return
}

// ExpectedStaticAssetMap builds a map keyed by filepath.
func ExpectedStaticAssetMap(t *testing.T, fsys fs.FS, root, uriPrefix string, filepaths ...string) map[string]ExpectedStaticAsset {
	t.Helper()
	expected := map[string]ExpectedStaticAsset{}
	for _, exp := range ExpectedStaticAssets(t, fsys, root, uriPrefix, filepaths...) {
		expected[exp.Filepath] = exp
	}
	return expected
}
