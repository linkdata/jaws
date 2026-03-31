package jawsboot_test

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

type expectedStaticAsset struct {
	filepath string
	uri      string
	plain    []byte
	ss       *staticserve.StaticServe
}

func readGzip(t *testing.T, b []byte) []byte {
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

func expectedStaticAssets(t *testing.T, fsys fs.FS, root, uriPrefix string) (expected []expectedStaticAsset) {
	t.Helper()
	var filepaths []string
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
			plain = readGzip(t, b)
		}
		expected = append(expected, expectedStaticAsset{
			filepath: filepath,
			uri:      path.Join(uriPrefix, ss.Name),
			plain:    plain,
			ss:       ss,
		})
	}
	return expected
}
