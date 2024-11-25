package templatereloader

import (
	"embed"
	"testing"
	"time"
)

//go:embed assets
var assetsFS embed.FS

func TestNew(t *testing.T) {
	tl, err := New(assetsFS, "assets/*.html", "")
	if err != nil {
		t.Fatal(err)
	}

	tr := tl.(*TemplateReloader)
	tr.when = tr.when.Add(-time.Second * 2)

	tmpl := tl.Lookup("test.html")
	if tmpl == nil {
		t.Fail()
	}
}

func Test_create_no_debug(t *testing.T) {
	tl, err := create(false, assetsFS, "assets/*.html", "")
	if err != nil {
		t.Fatal(err)
	}
	tmpl := tl.Lookup("test.html")
	if tmpl == nil {
		t.Fail()
	}
}
