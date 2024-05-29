package staticserve

import (
	"bytes"
	"compress/gzip"
	"hash/fnv"
	"mime"
	"path/filepath"
	"strconv"
	"strings"
)

type StaticServe struct {
	Name        string
	ContentType string
	Gz          []byte
}

func New(filename string, data []byte) (ss *StaticServe, err error) {
	var gz []byte
	if strings.HasSuffix(filename, ".gz") {
		gz = data
		filename = strings.TrimSuffix(filename, ".gz")
	} else {
		var buf bytes.Buffer
		gzw := gzip.NewWriter(&buf)
		defer gzw.Close()
		if _, err = gzw.Write(data); err == nil {
			if err = gzw.Flush(); err == nil {
				gz = buf.Bytes()
			}
		}
	}

	if err == nil {
		ext := filepath.Ext(filename)
		filename = strings.TrimSuffix(filename, ext)
		h := fnv.New64a()
		if _, err = h.Write(gz); err == nil {
			ss = &StaticServe{
				Name:        filename + "." + strconv.FormatUint(h.Sum64(), 36) + ext,
				ContentType: mime.TypeByExtension(ext),
				Gz:          gz,
			}
		}
	}

	return
}
