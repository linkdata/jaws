package templatereloader

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
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
	tr.path = "assets/this-matches-nothing-*.html"
	tr.when = tr.when.Add(-2 * time.Second)
	if tmpl := tr.Lookup("test.html"); tmpl == nil {
		t.Fatal("expected last-good template to be retained after a reload parse error")
	}
	if err := tr.LastError(); err == nil {
		t.Fatal("expected LastError after reload parse error")
	}

	tr.path = "assets/*.html"
	tr.when = tr.when.Add(-2 * time.Second)
	if tmpl := tr.Lookup("test.html"); tmpl == nil {
		t.Fatal("expected template after successful reload")
	}
	if err := tr.LastError(); err != nil {
		t.Fatalf("LastError after successful reload = %v, want nil", err)
	}
}

// Test_Lookup_failed_reload_backoff verifies the documented backoff: after a
// failed reload advances tr.when, a fix made within the interval window is not
// picked up until the window reopens.
func Test_Lookup_failed_reload_backoff(t *testing.T) {
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

	// Point at a glob matching nothing and force a reload so it fails, which
	// advances tr.when to now.
	tr.path = "assets/this-matches-nothing-*.html"
	tr.when = tr.when.Add(-2 * time.Second)
	if tmpl := tr.Lookup("test.html"); tmpl == nil {
		t.Fatal("expected last-good template after failed reload")
	}
	if err := tr.LastError(); err == nil {
		t.Fatal("expected LastError after failed reload")
	}

	// "Fix" the path but do not reopen the window. The next Lookup must not
	// reparse, so LastError stays set from the failed reload.
	tr.path = "assets/*.html"
	if tmpl := tr.Lookup("test.html"); tmpl == nil {
		t.Fatal("expected last-good template within the backoff window")
	}
	if err := tr.LastError(); err == nil {
		t.Fatal("reload should be deferred within the backoff window; LastError must remain set")
	}

	// Reopen the window: now the fix is picked up and LastError clears.
	tr.when = tr.when.Add(-2 * time.Second)
	if tmpl := tr.Lookup("test.html"); tmpl == nil {
		t.Fatal("expected template after the window reopened")
	}
	if err := tr.LastError(); err != nil {
		t.Fatalf("LastError after successful reload = %v, want nil", err)
	}
}

// TestTemplateReloader_ReloadPicksUpEditedContent verifies the package's headline
// behavior: after a template file is edited on disk and the reload window passes,
// Lookup serves the new content. It parses from a real temp dir (the embedded
// assets/test.html never changes, so it cannot exercise this), renders, rewrites
// the file with different content, forces the reload window, and asserts the
// rendered output changed — which fails if Lookup kept serving the stale tr.curr.
func TestTemplateReloader_ReloadPicksUpEditedContent(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "test.html")
	if err := os.WriteFile(tmplPath, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}

	tl, err := create(true, assetsFS, "*.html", dir)
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := tl.(*TemplateReloader)
	if !ok {
		t.Fatalf("expected *TemplateReloader, got %T", tl)
	}

	render := func() string {
		t.Helper()
		tmpl := tr.Lookup("test.html")
		if tmpl == nil {
			t.Fatal("expected template from lookup")
		}
		var sb strings.Builder
		if err := tmpl.Execute(&sb, nil); err != nil {
			t.Fatal(err)
		}
		return sb.String()
	}

	if got := render(); got != "v1" {
		t.Fatalf("initial render = %q, want %q", got, "v1")
	}

	if err := os.WriteFile(tmplPath, []byte("v2-edited"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Force the reload window so the next Lookup reparses from disk.
	tr.when = tr.when.Add(-2 * reloadInterval)
	if got := render(); got != "v2-edited" {
		t.Fatalf("post-edit render = %q, want %q", got, "v2-edited")
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

// TestTemplateReloader_ZeroValueLookupReturnsNil verifies the exported zero value
// is safe to use: it has never parsed any templates, so Lookup returns nil rather
// than dereferencing a nil *template.Template and panicking.
func TestTemplateReloader_ZeroValueLookupReturnsNil(t *testing.T) {
	tr := &TemplateReloader{}
	// The first call enters the reload path (tr.when is the zero time), fails to
	// parse the empty glob, and must return nil from the curr == nil guard.
	if tmpl := tr.Lookup("test.html"); tmpl != nil {
		t.Fatalf("zero-value Lookup = %v, want nil", tmpl)
	}
	// The first call advanced tr.when, so the second skips the reload and exercises
	// the curr == nil guard on the no-reload path; it must also return nil.
	if tmpl := tr.Lookup("test.html"); tmpl != nil {
		t.Fatalf("second zero-value Lookup = %v, want nil", tmpl)
	}
}

func TestTemplateReloader_Path(t *testing.T) {
	tl, err := create(true, assetsFS, "assets/*.html", "")
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := tl.(*TemplateReloader)
	if !ok {
		t.Fatalf("expected *TemplateReloader, got %T", tl)
	}
	if got := tr.Path(); got != "assets/*.html" {
		t.Errorf("Path() = %q, want %q", got, "assets/*.html")
	}

	var nilTR *TemplateReloader
	if got := nilTR.Path(); got != "" {
		t.Errorf("nil Path() = %q, want empty", got)
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
