package jaws

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"hash/fnv"
	"html/template"
	"strconv"
	"strings"
)

//go:embed jaws.js
var javascriptText []byte
var javascriptGZip []byte
var javascriptPath string

// JavascriptText returns the source code for the client-side JaWS Javascript library.
func JavascriptText() []byte {
	return javascriptText
}

// JavascriptGZip returns the embedded Javascript library GZipped.
func JavascriptGZip() []byte {
	if javascriptGZip == nil {
		b := bytes.Buffer{}
		gw := gzip.NewWriter(&b)
		_, err := gw.Write(JavascriptText())
		maybePanic(err)
		maybePanic(gw.Close())
		javascriptGZip = b.Bytes()
	}
	return javascriptGZip
}

// JavascriptPath returns the path for the embedded JaWS Javascript library.
func JavascriptPath() string {
	if javascriptPath == "" {
		h := fnv.New64a()
		_, err := h.Write(JavascriptText())
		maybePanic(err)
		javascriptPath = "/jaws/jaws." + strconv.FormatUint(h.Sum64(), 36) + ".js"
	}
	return javascriptPath
}

func appendLinkStyleSheet(s []byte, css string) []byte {
	s = append(s, `<link rel="stylesheet" href="`...)
	s = append(s, css...)
	s = append(s, `">`...)
	return s
}

// HeadHTML returns the HTML code to load the required CSS and Javascript
// libraries along with any `*.js“ and `*.css` URL's given in `extra`.
// Place the returned HTML code in the HEAD section of the document.
func HeadHTML(jawsKey uint64, extra ...string) template.HTML {
	const forEachPart = `.forEach(function(c){var e=document.createElement("script");e.src=c;e.async=!1;document.head.appendChild(e);});`
	keyStr := JawsKeyString(jawsKey)
	bsJS := bootstrapConfig.bootstrapJS
	bsCSS := bootstrapConfig.bootstrapCSS
	jawsJS := JavascriptPath()

	need := 21 + len(keyStr) + 4 + len(bsJS) + 3 + len(jawsJS) + 2 + len(forEachPart) + 9 + len(bsCSS) + 31 + 18
	for _, e := range extra {
		need += 3 + len(e)
		if strings.HasSuffix(e, ".css") {
			need += 31 - 3
		}
	}

	s := make([]byte, 0, need)
	s = append(s, `<script>var jawsKey="`...)
	s = append(s, keyStr...)
	s = append(s, `";["`...)
	s = append(s, bsJS...)
	s = append(s, `","`...)
	s = append(s, jawsJS...)
	for _, e := range extra {
		if strings.HasSuffix(e, ".js") {
			s = append(s, `","`...)
			s = append(s, e...)
		}
	}
	s = append(s, `"]`+forEachPart+`</script>`...)
	s = appendLinkStyleSheet(s, bsCSS)
	for _, e := range extra {
		if strings.HasSuffix(e, ".css") {
			s = appendLinkStyleSheet(s, e)
		}
	}
	s = append(s, `<script>0</script>`...) // forces FireFox to evaluate now
	return template.HTML(s)                // #nosec G203
}

func JawsKeyString(jawsKey uint64) string {
	if jawsKey == 0 {
		return ""
	}
	return strconv.FormatUint(jawsKey, 16)
}
