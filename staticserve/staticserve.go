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
	Name        string // the cache-busting file name, e.g. "static/filename.1234567.js"
	ContentType string // Content-Type of the file, e.g. "application/javascript"
	Gz          []byte // gzipped data, will be unpacked as needed
}

// New returns a StaticServe that serves the given data with a filename like 'filename.12345678.ext'.
// The filename must have the suffix ".gz" if the data is GZip compressed. The ".gz" suffix will
// not be part of the filename presented in this case.
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

func MaybePanic(err error) {
	if err != nil {
		panic(err)
	}
}

// Must calls New and panics on error.
func Must(filename string, data []byte) (ss *StaticServe) {
	var err error
	ss, err = New(filename, data)
	MaybePanic(err)
	return
}
