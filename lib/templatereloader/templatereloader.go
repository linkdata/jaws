package templatereloader

import (
	"html/template"
	"io/fs"
	"path"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
)

// defaultReloadInterval is the reload interval used by [New]. It is also the
// fallback for non-positive internal interval values.
const defaultReloadInterval = time.Second

// TemplateReloader reparses templates from disk at most once per second and is
// safe for concurrent use.
type TemplateReloader struct {
	// path is the glob templates are reparsed from. It is set once by create and
	// read under mu during a reload; [TemplateReloader.Path] exposes it read-only.
	// Keeping it unexported prevents callers from mutating it, which would race
	// with [TemplateReloader.Lookup].
	path string
	// interval is set during construction and immutable thereafter. A
	// non-positive value selects defaultReloadInterval, including for the zero
	// value.
	interval time.Duration
	mu       deadlock.RWMutex
	when     time.Time
	curr     *template.Template
	lastErr  error
}

// New returns a [jaws.TemplateLookuper] for the templates matched by fpath.
//
// In normal builds the templates are parsed once from fsys. When
// [deadlock.Debug] is set (race or debug builds) it instead returns a
// [TemplateReloader] that reparses from disk under relpath, so template edits take
// effect without a restart; fsys is then unused.
//
// The debug-build concrete type is observable via a type assertion to
// [*TemplateReloader], whose [TemplateReloader.LastError] and
// [TemplateReloader.Path] expose reload diagnostics; in normal builds the value is
// a plain *[html/template.Template] and there are no reload diagnostics to report.
//
// Because the two builds resolve templates differently, fpath must be a valid
// pattern under both [io/fs.Glob] (used against fsys in normal builds) and
// [path/filepath.Glob] (used against the on-disk tree in debug builds), and
// relpath must point at the on-disk root that mirrors the embedded fsys layout
// so that path.Join(relpath, fpath) matches the same templates on disk.
func New(fsys fs.FS, fpath, relpath string) (jaws.TemplateLookuper, error) {
	return create(deadlock.Debug, fsys, fpath, relpath, defaultReloadInterval)
}

func create(debug bool, fsys fs.FS, fpath, relpath string, interval time.Duration) (tl jaws.TemplateLookuper, err error) {
	if !debug {
		// Assign through a concrete local and only set the interface on success.
		// Returning template.New("").ParseFS(...) directly would, on a parse
		// error, yield a non-nil jaws.TemplateLookuper wrapping a nil
		// *template.Template, panicking any caller that checks tl != nil.
		var tmpl *template.Template
		if tmpl, err = template.New("").ParseFS(fsys, fpath); err == nil {
			tl = tmpl
		}
		return
	}
	var tmpl *template.Template
	fpath = path.Join(relpath, fpath)
	if tmpl, err = template.New("").ParseGlob(fpath); err == nil {
		tl = &TemplateReloader{
			path:     fpath,
			interval: interval,
			when:     time.Now(),
			curr:     tmpl,
		}
	}
	return
}

// Lookup returns the named template, reparsing from disk when the reload
// interval has elapsed.
//
// If reparsing fails, Lookup records the error for [TemplateReloader.LastError]
// and retains the last successful templates. It does not retry until another
// interval elapses.
//
// The zero value returns nil.
func (tr *TemplateReloader) Lookup(name string) *template.Template {
	tr.mu.RLock()
	curr := tr.curr
	interval := tr.interval
	if interval <= 0 {
		interval = defaultReloadInterval
	}
	d := time.Since(tr.when)
	tr.mu.RUnlock()
	if d > interval {
		tr.mu.Lock()
		// Re-check under the write lock so concurrent callers that all
		// observed a stale time do not each reparse from disk.
		if time.Since(tr.when) > interval {
			reloaded, err := template.New("").ParseGlob(tr.path)
			tr.lastErr = err
			if err == nil {
				tr.curr = reloaded
			}
			tr.when = time.Now()
		}
		curr = tr.curr
		tr.mu.Unlock()
	}
	if curr == nil {
		// The zero value has no parsed templates (and an empty path never parses
		// any), so there is nothing to look up; return nil rather than dereferencing
		// a nil *template.Template.
		return nil
	}
	return curr.Lookup(name)
}

// LastError returns the last reload parse error, or nil after a successful reload.
//
// It is safe to call on a nil *TemplateReloader.
func (tr *TemplateReloader) LastError() (err error) {
	// lastErr is the raw parse error and is non-nil only after a failed reload, so
	// LastError() != nil is itself the "did the last reload fail" predicate. There is
	// no other error source to tell apart, so the error needs no wrapping sentinel to
	// match against; returning it unwrapped keeps the parse detail intact.
	if tr != nil {
		tr.mu.RLock()
		err = tr.lastErr
		tr.mu.RUnlock()
	}
	return
}

// Path returns the glob pattern templates are reparsed from.
//
// It is safe to call on a nil *TemplateReloader, in which case it returns "".
func (tr *TemplateReloader) Path() (s string) {
	if tr != nil {
		tr.mu.RLock()
		s = tr.path
		tr.mu.RUnlock()
	}
	return
}
