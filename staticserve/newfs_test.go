package staticserve

import (
	"embed"
	"testing"
)

//go:embed assets
var assetsFS embed.FS

func TestNewFS(t *testing.T) {
	ss, err := NewFS(assetsFS, "assets", "subdir/test.txt")
	if err != nil {
		t.Error(err)
	}
	if ss.ContentType != "text/plain; charset=utf-8" {
		t.Error(ss.ContentType)
	}
	if ss.Name != "subdir/test.u9cvw0b8o4xe.txt" {
		t.Error(ss.Name)
	}
}

func TestMustNewFS(t *testing.T) {
	ssl := MustNewFS(assetsFS, "assets", "subdir/test.txt")
	if len(ssl) != 1 {
		t.Error(ssl)
	}
}
