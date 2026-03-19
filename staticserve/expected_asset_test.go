package staticserve_test

import (
	"bytes"
	"compress/gzip"
	"embed"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/linkdata/jaws/staticserve"
)

//go:embed assets
var assetsFS embed.FS

type expectedAsset struct {
	Filepath    string
	Name        string
	URI         string
	ContentType string
	Plain       []byte
	Gz          []byte
}

func assetFilepaths(t *testing.T) (filepaths []string) {
	t.Helper()
	err := fs.WalkDir(assetsFS, "assets", func(pathname string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		filepaths = append(filepaths, strings.TrimPrefix(pathname, "assets/"))
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

func expectedAssets(t *testing.T, filepaths ...string) (expected []expectedAsset) {
	t.Helper()
	if len(filepaths) == 0 {
		filepaths = assetFilepaths(t)
	}
	for _, filepath := range filepaths {
		b, err := fs.ReadFile(assetsFS, path.Join("assets", filepath))
		if err != nil {
			t.Fatal(err)
		}
		ss, err := staticserve.New(filepath, b)
		if err != nil {
			t.Fatal(err)
		}
		plain := b
		if strings.HasSuffix(filepath, ".gz") {
			gzr, err := gzip.NewReader(bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			plain, err = io.ReadAll(gzr)
			if cerr := gzr.Close(); cerr != nil && err == nil {
				err = cerr
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		expected = append(expected, expectedAsset{
			Filepath:    filepath,
			Name:        ss.Name,
			URI:         "/" + ss.Name,
			ContentType: ss.ContentType,
			Plain:       plain,
			Gz:          ss.Gz,
		})
	}
	return
}

func expectedAssetMap(t *testing.T, filepaths ...string) (expected map[string]expectedAsset) {
	t.Helper()
	expected = map[string]expectedAsset{}
	for _, exp := range expectedAssets(t, filepaths...) {
		expected[exp.Filepath] = exp
	}
	return
}
