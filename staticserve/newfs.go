package staticserve

import (
	"io"
	"io/fs"
	"path/filepath"
)

// NewFS reads the file at fpath from fsys and then calls New with
// the filename part of fpath.
func NewFS(fsys fs.FS, fpath string) (ss *StaticServe, err error) {
	var f fs.File
	if f, err = fsys.Open(fpath); err == nil {
		defer f.Close()
		var b []byte
		if b, err = io.ReadAll(f); err == nil {
			ss, err = New(filepath.Base(fpath), b)
		}
	}
	return
}
