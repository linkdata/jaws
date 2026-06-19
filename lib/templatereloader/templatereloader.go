package templatereloader

import (
	"html/template"
	"io/fs"
	"path"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
)

// reloadInterval is the minimum time between disk reparses in debug mode.
const reloadInterval = time.Second

// TemplateReloader reloads and reparses templates if more than one second
// has passed since the last reload.
type TemplateReloader struct {
	// Path is the file path templates are loaded from. It is set once by [New]
	// and is read-only afterwards: it is read under mu during a reload, so
	// mutating it after construction would race with [TemplateReloader.Lookup].
	Path    string
	mu      deadlock.RWMutex
	when    time.Time
	curr    *template.Template
	lastErr error
}

// New returns a [jaws.TemplateLookuper] for the templates matched by fpath.
//
// In normal builds the templates are parsed once from fsys. In debug builds
// (deadlock.Debug, set by -race or -tags debug) it instead returns a
// [TemplateReloader] that reparses from disk under relpath, so template edits take
// effect without a restart; fsys is then unused.
func New(fsys fs.FS, fpath, relpath string) (jtl jaws.TemplateLookuper, err error) {
	return create(deadlock.Debug, fsys, fpath, relpath)
}

func create(debug bool, fsys fs.FS, fpath, relpath string) (tl jaws.TemplateLookuper, err error) {
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
			Path: fpath,
			when: time.Now(),
			curr: tmpl,
		}
	}
	return
}

// Lookup returns the named template, reparsing the templates from disk first
// when more than one second has passed since the last reload.
//
// If a reload fails to parse (for example while a template file is being
// edited), the last successfully parsed templates are retained and used, so a
// transient parse error does not take down a running server. Lookup never
// panics on a reload error.
func (tr *TemplateReloader) Lookup(name string) *template.Template {
	tr.mu.RLock()
	curr := tr.curr
	d := time.Since(tr.when)
	tr.mu.RUnlock()
	if d > reloadInterval {
		tr.mu.Lock()
		// Re-check under the write lock so concurrent callers that all
		// observed a stale time do not each reparse from disk.
		if time.Since(tr.when) > reloadInterval {
			if reloaded, err := template.New("").ParseGlob(tr.Path); err == nil {
				tr.curr = reloaded
				tr.lastErr = nil
			} else {
				tr.lastErr = err
			}
			tr.when = time.Now()
		}
		curr = tr.curr
		tr.mu.Unlock()
	}
	return curr.Lookup(name)
}

// LastError returns the last reload parse error, or nil after a successful reload.
//
// It is safe to call on a nil *TemplateReloader.
func (tr *TemplateReloader) LastError() (err error) {
	if tr != nil {
		tr.mu.RLock()
		err = tr.lastErr
		tr.mu.RUnlock()
	}
	return
}
