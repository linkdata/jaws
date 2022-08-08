package jaws

import (
	"strings"
	"testing"

	"github.com/matryer/is"
)

func Test_Javascript(t *testing.T) {
	const prefix = "/jaws/jaws."
	const suffix = ".js"
	is := is.New(t)
	path := JavascriptPath()
	is.True(strings.HasPrefix(path, prefix))
	is.True(strings.HasSuffix(path, suffix))
	is.True(len(path) > len(prefix)+len(suffix))
	text := JavascriptText()
	is.True(len(text) > 0)
	gzip := JavascriptGZip()
	is.True(len(gzip) > 0)
	is.True(len(gzip) < len(text))
}

func Test_HeadHTML(t *testing.T) {
	const extraScript = "someExtraScript.js"
	is := is.New(t)
	jawsKey := uint64(0xcafebabe)
	txt := HeadHTML(jawsKey, []string{extraScript})
	is.Equal(strings.Contains(string(txt), JawsKeyString(jawsKey)), true)
	is.Equal(strings.Contains(string(txt), JavascriptPath()), true)
	is.Equal(strings.Contains(string(txt), extraScript), true)
}
