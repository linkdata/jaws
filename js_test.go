package jaws

import (
	"bytes"
	"compress/gzip"
	"hash/fnv"
	"strconv"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func Test_Javascript(t *testing.T) {
	const prefix = "/jaws/jaws."
	const suffix = ".js"
	is := is.New(t)

	h := fnv.New64a()
	_, err := h.Write(JavascriptText)
	is.NoErr(err)
	is.Equal(JavascriptPath, prefix+strconv.FormatUint(h.Sum64(), 36)+suffix)

	is.True(len(JavascriptText) > 0)
	is.True(len(JavascriptGZip) > 0)
	is.True(len(JavascriptGZip) < len(JavascriptText))

	b := bytes.Buffer{}
	gw := gzip.NewWriter(&b)
	_, err = gw.Write(JavascriptText)
	is.NoErr(err)
	is.NoErr(gw.Close())
	is.Equal(b.Bytes(), JavascriptGZip)
}

func Test_HeadHTML(t *testing.T) {
	const extraScript = "someExtraScript.js"
	const extraStyle = "someExtraStyle.css"
	is := is.New(t)
	txt := HeadHTML(nil, nil)
	is.Equal(strings.Contains(string(txt), JavascriptPath), false)
	txt = HeadHTML([]string{JavascriptPath, extraScript}, []string{extraStyle})
	is.Equal(strings.Contains(string(txt), JavascriptPath), true)
	is.Equal(strings.Contains(string(txt), extraScript), true)
	is.Equal(strings.Contains(string(txt), extraStyle), true)
}
