package staticserve

import (
	"io"
	"io/fs"
	"strings"
)

// WalkDir walks the file tree rooted at root, calling fn for each file in the tree with
// the filename having root trimmed (e.g. "root/dir/file.ext" becomes "dir/file.ext").
func WalkDir(fsys fs.FS, root string, fn func(filename string, ss *StaticServe) (err error)) (err error) {
	err = fs.WalkDir(fsys, root, func(filename string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			var f fs.File
			if f, err = fsys.Open(filename); err == nil {
				defer f.Close()
				var b []byte
				if b, err = io.ReadAll(f); err == nil {
					var ss *StaticServe
					filename = strings.TrimPrefix(strings.TrimPrefix(filename, root), "/")
					if ss, err = New(filename, b); err == nil {
						err = fn(filename, ss)
					}
				}
			}
		}
		return err
	})
	return
}
