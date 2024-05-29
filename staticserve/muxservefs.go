package staticserve

import (
	"io"
	"io/fs"
	"net/http"
	"path"
)

func MuxServeFS(mux *http.ServeMux, prefix string, fsys fs.FS) (uris map[string]string, err error) {
	err = fs.WalkDir(fsys, ".", func(fn string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			var f fs.File
			if f, err = fsys.Open(fn); err == nil {
				defer f.Close()
				var b []byte
				if b, err = io.ReadAll(f); err == nil {
					var ss *StaticServe
					if ss, err = New(fn, b); err == nil {
						uri := path.Join(prefix, ss.Name)
						if uris == nil {
							uris = make(map[string]string)
						}
						uris[fn] = uri
						mux.Handle(uri, ss)
					}
				}
			}
		}
		return err
	})
	return
}
