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

	UseBootstrap(nil)
	is.True(bootstrapConfig.bootstrapJS != "")
	is.True(bootstrapConfig.bootstrapCSS != "")

	jawsKey := uint64(0xcafebabe)
	txt := HeadHTML(jawsKey)
	is.Equal(strings.Contains(string(txt), JawsKeyString(jawsKey)), true)
	is.Equal(strings.Contains(string(txt), JavascriptPath), true)
	is.Equal(strings.Contains(string(txt), bootstrapConfig.bootstrapJS), true)
	is.Equal(strings.Contains(string(txt), bootstrapConfig.bootstrapCSS), true)
	txt = HeadHTML(jawsKey, extraScript, extraStyle)
	is.Equal(strings.Contains(string(txt), JawsKeyString(jawsKey)), true)
	is.Equal(strings.Contains(string(txt), JavascriptPath), true)
	is.Equal(strings.Contains(string(txt), bootstrapConfig.bootstrapJS), true)
	is.Equal(strings.Contains(string(txt), bootstrapConfig.bootstrapCSS), true)
	is.Equal(strings.Contains(string(txt), extraScript), true)
	is.Equal(strings.Contains(string(txt), extraStyle), true)
}
