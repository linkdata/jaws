package jaws

import (
	"fmt"
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
	const extraStyle = "someExtraStyle.css"
	is := is.New(t)

	UseBootstrap(nil)
	is.True(bootstrapConfig.bootstrapJS != "")
	is.True(bootstrapConfig.bootstrapCSS != "")
	fmt.Println(bootstrapConfig.bootstrapJS)
	fmt.Println(bootstrapConfig.bootstrapCSS)

	jawsKey := uint64(0xcafebabe)
	txt := HeadHTML(jawsKey)
	is.Equal(strings.Contains(string(txt), JawsKeyString(jawsKey)), true)
	is.Equal(strings.Contains(string(txt), JavascriptPath()), true)
	is.Equal(strings.Contains(string(txt), bootstrapConfig.bootstrapJS), true)
	is.Equal(strings.Contains(string(txt), bootstrapConfig.bootstrapCSS), true)
	txt = HeadHTML(jawsKey, extraScript, extraStyle)
	is.Equal(strings.Contains(string(txt), JawsKeyString(jawsKey)), true)
	is.Equal(strings.Contains(string(txt), JavascriptPath()), true)
	is.Equal(strings.Contains(string(txt), bootstrapConfig.bootstrapJS), true)
	is.Equal(strings.Contains(string(txt), bootstrapConfig.bootstrapCSS), true)
	is.Equal(strings.Contains(string(txt), extraScript), true)
	is.Equal(strings.Contains(string(txt), extraStyle), true)
	fmt.Println(string(txt))
}
