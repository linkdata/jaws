package templatereloader

import (
	"embed"
	"sync"
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

func Test_Lookup_reload_error_retains_last_good(t *testing.T) {
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

	// Point at a glob that matches no files so the next reload fails to parse,
	// then force a reload. Lookup must not panic and must keep serving the
	// last successfully parsed template.
	tr.Path = "assets/this-matches-nothing-*.html"
	tr.when = tr.when.Add(-2 * time.Second)
	if tmpl := tr.Lookup("test.html"); tmpl == nil {
		t.Fatal("expected last-good template to be retained after a reload parse error")
	}
	if err := tr.LastError(); err == nil {
		t.Fatal("expected LastError after reload parse error")
	}

	tr.Path = "assets/*.html"
	tr.when = tr.when.Add(-2 * time.Second)
	if tmpl := tr.Lookup("test.html"); tmpl == nil {
		t.Fatal("expected template after successful reload")
	}
	if err := tr.LastError(); err != nil {
		t.Fatalf("LastError after successful reload = %v, want nil", err)
	}
}

func TestTemplateReloader_LastErrorNilReceiver(t *testing.T) {
	var tr *TemplateReloader
	if err := tr.LastError(); err != nil {
		t.Fatalf("nil LastError = %v, want nil", err)
	}
}

// TestTemplateReloader_ConcurrentLookup runs many Lookups concurrently after
// forcing a reload window, exercising the double-checked locking under
// contention. Run with -race to validate the locking.
func TestTemplateReloader_ConcurrentLookup(t *testing.T) {
	tl, err := create(true, assetsFS, "assets/*.html", "")
	if err != nil {
		t.Fatal(err)
	}
	tr := tl.(*TemplateReloader)

	// Force the reload window so all goroutines hit the reparse path together;
	// the re-check under the write lock must let only one of them reparse.
	tr.mu.Lock()
	tr.when = tr.when.Add(-2 * time.Second)
	tr.mu.Unlock()

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range 50 {
				if tmpl := tr.Lookup("test.html"); tmpl == nil {
					t.Error("expected template from concurrent lookup")
				}
			}
		}()
	}
	wg.Wait()
	if err := tr.LastError(); err != nil {
		t.Fatalf("unexpected reload error: %v", err)
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

func Test_create_no_debug_parse_error(t *testing.T) {
	tl, err := create(false, assetsFS, "assets/missing-*.html", "")
	if err == nil {
		t.Fatal("expected parse error")
	}
	// The non-debug path must return a true nil interface on error, not a
	// non-nil jaws.TemplateLookuper wrapping a nil *template.Template.
	if tl != nil {
		t.Fatalf("expected nil lookuper on error, got %T", tl)
	}
}

func TestNew_parse_error_returns_nil_lookuper(t *testing.T) {
	tl, err := New(assetsFS, "assets/missing-*.html", "")
	if err == nil {
		t.Fatal("expected parse error")
	}
	// On a parse error the returned interface must be a true nil in both debug
	// and non-debug modes, so callers can rely on tl != nil meaning success.
	if tl != nil {
		t.Fatalf("expected nil lookuper on parse error, got %T", tl)
	}
}
