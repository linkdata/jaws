package staticserve

import (
	"io"
	"io/fs"
	"path"
)

// NewFS reads the file at fpath from fsys and then calls New.
func NewFS(fsys fs.FS, root, fpath string) (ss *StaticServe, err error) {
	var f fs.File
	if f, err = fsys.Open(path.Join(root, fpath)); err == nil {
		defer f.Close()
		var b []byte
		if b, err = io.ReadAll(f); err == nil {
			ss, err = New(fpath, b)
		}
	}
	return
}

func MustNewFS(fsys fs.FS, root string, fpaths ...string) (ssl []*StaticServe) {
	for _, fpath := range fpaths {
		ss, err := NewFS(fsys, root, fpath)
		MaybePanic(err)
		ssl = append(ssl, ss)
	}
	return
}
