package templatereloader

import (
	"html/template"
	"io/fs"
	"path"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
)

// A TemplateReloader reloads and reparses templates if more than one second
// has passed since the last Lookup.
type TemplateReloader struct {
	Path string // the file path we are loading from
	mu   deadlock.RWMutex
	when time.Time
	curr *template.Template
}

// New returns a jaws.TemplateLookuper.
//
// If deadlock.Debug is false, it calls template.New("").ParseFS(fsys, fpath).
//
// If deadlock.Debug is true, fsys is ignored and it returns a TemplateReloader
// that loads the templates using ParseGlob(relpath/fpath).
func New(fsys fs.FS, fpath, relpath string) (jtl jaws.TemplateLookuper, err error) {
	return create(deadlock.Debug, fsys, fpath, relpath)
}

func create(debug bool, fsys fs.FS, fpath, relpath string) (tl jaws.TemplateLookuper, err error) {
	if !debug {
		return template.New("").ParseFS(fsys, fpath)
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

func (tr *TemplateReloader) Lookup(name string) *template.Template {
	tr.mu.RLock()
	tl := tr.curr
	d := time.Since(tr.when)
	tr.mu.RUnlock()
	if d > time.Second {
		tr.mu.Lock()
		defer tr.mu.Unlock()
		tr.curr = template.Must(template.New("").ParseGlob(tr.Path))
		tr.when = tr.when.Add(d)
		tl = tr.curr
	}
	return tl.Lookup(name)
}
