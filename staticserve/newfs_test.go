package staticserve_test

import (
	"bytes"
	"testing"

	"github.com/linkdata/jaws/staticserve"
)

func TestNewFS(t *testing.T) {
	for _, exp := range expectedAssets(t) {
		ss, err := staticserve.NewFS(assetsFS, "assets", exp.Filepath)
		if err != nil {
			t.Fatal(err)
		}
		if ss == nil {
			t.Fatalf("%q: nil StaticServe", exp.Filepath)
		}
		if ss.Name != exp.Name {
			t.Errorf("%q: expected name %q, got %q", exp.Filepath, exp.Name, ss.Name)
		}
		if ss.ContentType != exp.ContentType {
			t.Errorf("%q: expected content type %q, got %q", exp.Filepath, exp.ContentType, ss.ContentType)
		}
		if !bytes.Equal(ss.Gz, exp.Gz) {
			t.Errorf("%q: gz payload mismatch", exp.Filepath)
		}
	}
}

func TestMustNewFS(t *testing.T) {
	expected := expectedAssets(t)
	filepaths := make([]string, 0, len(expected))
	for _, exp := range expected {
		filepaths = append(filepaths, exp.Filepath)
	}

	got := staticserve.MustNewFS(assetsFS, "assets", filepaths...)
	if len(got) != len(expected) {
		t.Fatalf("expected %d StaticServe values, got %d", len(expected), len(got))
	}
	for i := range expected {
		if got[i] == nil {
			t.Fatalf("%q: nil StaticServe", expected[i].Filepath)
		}
		if got[i].Name != expected[i].Name {
			t.Errorf("%q: expected name %q, got %q", expected[i].Filepath, expected[i].Name, got[i].Name)
		}
		if got[i].ContentType != expected[i].ContentType {
			t.Errorf("%q: expected content type %q, got %q", expected[i].Filepath, expected[i].ContentType, got[i].ContentType)
		}
		if !bytes.Equal(got[i].Gz, expected[i].Gz) {
			t.Errorf("%q: gz payload mismatch", expected[i].Filepath)
		}
	}
}
