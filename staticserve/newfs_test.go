package staticserve_test

import (
	"bytes"
	"testing"

	"github.com/linkdata/jaws/staticserve"
)

func TestNewFS(t *testing.T) {
	for _, exp := range expectedStaticAssets(t, assetsFS, "assets", "/") {
		ss, err := staticserve.NewFS(assetsFS, "assets", exp.filepath)
		if err != nil {
			t.Fatal(err)
		}
		if ss == nil {
			t.Fatalf("%q: nil StaticServe", exp.filepath)
		}
		if ss.Name != exp.name {
			t.Errorf("%q: expected name %q, got %q", exp.filepath, exp.name, ss.Name)
		}
		if ss.ContentType != exp.contentType {
			t.Errorf("%q: expected content type %q, got %q", exp.filepath, exp.contentType, ss.ContentType)
		}
		if !bytes.Equal(ss.Gz, exp.gz) {
			t.Errorf("%q: gz payload mismatch", exp.filepath)
		}
	}
}

func TestMustNewFS(t *testing.T) {
	expected := expectedStaticAssets(t, assetsFS, "assets", "/")
	filepaths := make([]string, 0, len(expected))
	for _, exp := range expected {
		filepaths = append(filepaths, exp.filepath)
	}

	got := staticserve.MustNewFS(assetsFS, "assets", filepaths...)
	if len(got) != len(expected) {
		t.Fatalf("expected %d StaticServe values, got %d", len(expected), len(got))
	}
	for i := range expected {
		if got[i] == nil {
			t.Fatalf("%q: nil StaticServe", expected[i].filepath)
		}
		if got[i].Name != expected[i].name {
			t.Errorf("%q: expected name %q, got %q", expected[i].filepath, expected[i].name, got[i].Name)
		}
		if got[i].ContentType != expected[i].contentType {
			t.Errorf("%q: expected content type %q, got %q", expected[i].filepath, expected[i].contentType, got[i].ContentType)
		}
		if !bytes.Equal(got[i].Gz, expected[i].gz) {
			t.Errorf("%q: gz payload mismatch", expected[i].filepath)
		}
	}
}
