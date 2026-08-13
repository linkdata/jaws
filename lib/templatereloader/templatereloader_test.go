package templatereloader

import (
	"embed"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

//go:embed assets
var assetsFS embed.FS

const testReloadInterval = 10 * time.Millisecond

func createTestReloader(t *testing.T, fpath, relpath string, interval time.Duration) *TemplateReloader {
	t.Helper()
	tl, err := create(true, assetsFS, fpath, relpath, interval)
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := tl.(*TemplateReloader)
	if !ok {
		t.Fatalf("expected *TemplateReloader, got %T", tl)
	}
	return tr
}

func createFileReloader(t *testing.T, interval time.Duration, content string) (*TemplateReloader, string) {
	t.Helper()
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "test.html")
	writeTemplateFile(t, tmplPath, content)
	return createTestReloader(t, "*.html", dir, interval), tmplPath
}

func writeTemplateFile(t *testing.T, tmplPath, content string) {
	t.Helper()
	if err := os.WriteFile(tmplPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func renderTemplate(t *testing.T, tr *TemplateReloader) string {
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

func TestNew(t *testing.T) {
	tl, err := New(assetsFS, "assets/*.html", "")
	if err != nil {
		t.Fatal(err)
	}
	if tmpl := tl.Lookup("test.html"); tmpl == nil {
		t.Fatal("expected template from lookup")
	}
	if tr, ok := tl.(*TemplateReloader); ok && tr.interval != defaultReloadInterval {
		t.Fatalf("New reload interval = %v, want %v", tr.interval, defaultReloadInterval)
	}
}

func Test_create_no_debug(t *testing.T) {
	tl, err := create(false, assetsFS, "assets/*.html", "", defaultReloadInterval)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := tl.Lookup("test.html")
	if tmpl == nil {
		t.Fail()
	}
}

func Test_create_debug_and_lookup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr := createTestReloader(t, "assets/*.html", "", testReloadInterval)

		first := tr.Lookup("test.html")
		if first == nil {
			t.Fatal("expected template from first lookup")
		}
		if tmpl := tr.Lookup("test.html"); tmpl != first {
			t.Fatal("lookup reloaded before the configured interval elapsed")
		}

		time.Sleep(testReloadInterval + time.Nanosecond)
		if tmpl := tr.Lookup("test.html"); tmpl == nil {
			t.Fatal("expected template from reload lookup")
		} else if tmpl == first {
			t.Fatal("lookup did not reload after the configured interval elapsed")
		}
	})
}

func Test_Lookup_reload_error_retains_last_good(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr, tmplPath := createFileReloader(t, testReloadInterval, "v1")
		if got := renderTemplate(t, tr); got != "v1" {
			t.Fatalf("initial render = %q, want %q", got, "v1")
		}

		writeTemplateFile(t, tmplPath, "{{")
		time.Sleep(testReloadInterval + time.Nanosecond)
		if got := renderTemplate(t, tr); got != "v1" {
			t.Fatalf("render after failed reload = %q, want last-good %q", got, "v1")
		}
		if err := tr.LastError(); err == nil {
			t.Fatal("expected LastError after reload parse error")
		}

		writeTemplateFile(t, tmplPath, "v2")
		time.Sleep(testReloadInterval + time.Nanosecond)
		if got := renderTemplate(t, tr); got != "v2" {
			t.Fatalf("render after successful reload = %q, want %q", got, "v2")
		}
		if err := tr.LastError(); err != nil {
			t.Fatalf("LastError after successful reload = %v, want nil", err)
		}
	})
}

// Test_Lookup_failed_reload_backoff verifies the documented backoff: after a
// failed reload starts a new interval window, a fix made within that window is
// not picked up until the window reopens.
func Test_Lookup_failed_reload_backoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr, tmplPath := createFileReloader(t, testReloadInterval, "v1")

		writeTemplateFile(t, tmplPath, "{{")
		time.Sleep(testReloadInterval + time.Nanosecond)
		if got := renderTemplate(t, tr); got != "v1" {
			t.Fatalf("render after failed reload = %q, want last-good %q", got, "v1")
		}
		if err := tr.LastError(); err == nil {
			t.Fatal("expected LastError after failed reload")
		}

		// Repairing the file within the backoff window must not trigger another
		// reparse or clear the previous error.
		writeTemplateFile(t, tmplPath, "v2")
		if got := renderTemplate(t, tr); got != "v1" {
			t.Fatalf("render within backoff window = %q, want last-good %q", got, "v1")
		}
		if err := tr.LastError(); err == nil {
			t.Fatal("reload within backoff window cleared LastError")
		}

		time.Sleep(testReloadInterval + time.Nanosecond)
		if got := renderTemplate(t, tr); got != "v2" {
			t.Fatalf("render after backoff window = %q, want %q", got, "v2")
		}
		if err := tr.LastError(); err != nil {
			t.Fatalf("LastError after successful reload = %v, want nil", err)
		}
	})
}

// TestTemplateReloader_ReloadPicksUpEditedContent verifies the package's headline
// behavior: after a template file is edited on disk and the reload window passes,
// Lookup serves the new content. It parses from a real temp dir (the embedded
// assets/test.html never changes, so it cannot exercise this), renders, rewrites
// the file with different content, advances past the reload window, and asserts
// the rendered output changed.
func TestTemplateReloader_ReloadPicksUpEditedContent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr, tmplPath := createFileReloader(t, testReloadInterval, "v1")
		if got := renderTemplate(t, tr); got != "v1" {
			t.Fatalf("initial render = %q, want %q", got, "v1")
		}

		writeTemplateFile(t, tmplPath, "v2-edited")
		if got := renderTemplate(t, tr); got != "v1" {
			t.Fatalf("render before reload interval = %q, want %q", got, "v1")
		}

		time.Sleep(testReloadInterval + time.Nanosecond)
		if got := renderTemplate(t, tr); got != "v2-edited" {
			t.Fatalf("post-edit render = %q, want %q", got, "v2-edited")
		}
		if err := tr.LastError(); err != nil {
			t.Fatalf("LastError after successful reload = %v, want nil", err)
		}
	})
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
	// The first call attempts to reload the empty glob and must return nil rather
	// than dereferencing a nil template.
	if tmpl := tr.Lookup("test.html"); tmpl != nil {
		t.Fatalf("zero-value Lookup = %v, want nil", tmpl)
	}
	// Repeated use must preserve the documented nil result.
	if tmpl := tr.Lookup("test.html"); tmpl != nil {
		t.Fatalf("second zero-value Lookup = %v, want nil", tmpl)
	}
}

func TestTemplateReloader_NonPositiveIntervalUsesDefault(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
	}{
		{name: "zero", interval: 0},
		{name: "negative", interval: -time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				tr, tmplPath := createFileReloader(t, tt.interval, "v1")
				writeTemplateFile(t, tmplPath, "v2")

				time.Sleep(defaultReloadInterval - time.Nanosecond)
				if got := renderTemplate(t, tr); got != "v1" {
					t.Fatalf("render before default interval = %q, want %q", got, "v1")
				}

				time.Sleep(2 * time.Nanosecond)
				if got := renderTemplate(t, tr); got != "v2" {
					t.Fatalf("render after default interval = %q, want %q", got, "v2")
				}
			})
		})
	}
}

func TestTemplateReloader_Path(t *testing.T) {
	tl, err := create(true, assetsFS, "assets/*.html", "", defaultReloadInterval)
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

// TestTemplateReloader_ConcurrentLookup runs many Lookups concurrently after a
// reload window, exercising the double-checked locking under contention. Run
// with -race to validate the locking.
func TestTemplateReloader_ConcurrentLookup(t *testing.T) {
	// Real goroutines are used deliberately: a synctest bubble serializes them
	// and would hide the lock contention this test exercises.
	const interval = 100 * time.Millisecond
	tr := createTestReloader(t, "assets/*.html", "", interval)
	before := tr.Lookup("test.html")
	if before == nil {
		t.Fatal("expected template before concurrent reload")
	}

	// Hold the write lock while the interval elapses and callers queue for their
	// optimistic reads. Once released, every reader observes the stale timestamp
	// before any can acquire the write lock. The re-check under that lock must let
	// only one caller reparse.
	tr.mu.Lock()
	time.Sleep(interval + time.Millisecond)
	const goroutines = 16
	first := make([]*template.Template, goroutines)
	var ready, firstDone, wg sync.WaitGroup
	ready.Add(goroutines)
	firstDone.Add(goroutines)
	wg.Add(goroutines)
	for i := range goroutines {
		go func() {
			defer wg.Done()
			ready.Done()
			first[i] = tr.Lookup("test.html")
			firstDone.Done()
			firstDone.Wait()
			for range 50 {
				if tmpl := tr.Lookup("test.html"); tmpl == nil {
					t.Error("expected template from concurrent lookup")
				}
			}
		}()
	}
	ready.Wait()
	time.Sleep(20 * time.Millisecond)
	tr.mu.Unlock()
	wg.Wait()
	if first[0] == nil {
		t.Fatal("expected template from first concurrent lookup")
	}
	for i, tmpl := range first[1:] {
		if tmpl != first[0] {
			t.Fatalf("concurrent lookup %d returned a different parsed template set", i+1)
		}
	}
	if first[0] == before {
		t.Fatal("concurrent lookup did not reload the parsed template set")
	}
	if err := tr.LastError(); err != nil {
		t.Fatalf("unexpected reload error: %v", err)
	}
}

func Test_create_debug_parse_error(t *testing.T) {
	tl, err := create(true, assetsFS, "assets/missing-*.html", "", defaultReloadInterval)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if tl != nil {
		t.Fatalf("expected nil lookuper on error, got %T", tl)
	}
}

func Test_create_no_debug_parse_error(t *testing.T) {
	tl, err := create(false, assetsFS, "assets/missing-*.html", "", defaultReloadInterval)
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
