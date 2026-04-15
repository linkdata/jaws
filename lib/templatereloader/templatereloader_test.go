package templatereloader

import (
	"embed"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
)

//go:embed assets
var assetsFS embed.FS

func TestNew(t *testing.T) {
	tl, err := New(assetsFS, "assets/*.html", "")
	if err != nil {
		t.Fatal(err)
	}

	tr, ok := tl.(*TemplateReloader)
	if ok {
		tr.when = tr.when.Add(-time.Second * 2)

		tmpl := tl.Lookup("test.html")
		if tmpl == nil {
			t.Fail()
		}
	} else {
		t.Skip("not running with debug tag")
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

func Test_create_debug_and_lookup(t *testing.T) {
	tl, err := create(true, assetsFS, "assets/*.html", "")
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := tl.(*TemplateReloader)
	if !ok {
		t.Fatalf("expected *TemplateReloader, got %T", tl)
	}

	if tmpl := tr.Lookup("test.html"); tmpl == nil {
		t.Fatal("expected template from first lookup")
	}

	tr.when = tr.when.Add(-2 * time.Second)
	if tmpl := tr.Lookup("test.html"); tmpl == nil {
		t.Fatal("expected template from reload lookup")
	}
}

func Test_create_debug_parse_error(t *testing.T) {
	tl, err := create(true, assetsFS, "assets/missing-*.html", "")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if tl != nil {
		t.Fatalf("expected nil lookuper on error, got %T", tl)
	}
}

func TestNew_parse_error_passthrough(t *testing.T) {
	tl, err := New(assetsFS, "assets/missing-*.html", "")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if deadlock.Debug {
		if tl != nil {
			t.Fatalf("expected nil lookuper from debug-mode ParseGlob, got %T", tl)
		}
	} else if tl == nil {
		t.Fatal("expected non-nil lookuper from ParseFS, got nil")
	}
}
