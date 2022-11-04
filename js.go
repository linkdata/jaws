package jaws

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"hash/fnv"
	"html/template"
	"strconv"
)

// JavascriptText is the source code for the client-side JaWS Javascript library.
//
//go:embed jaws.js
var JavascriptText []byte

// JavascriptGZip is the embedded Javascript library GZipped.
var JavascriptGZip = makeJavascriptGZip()

// JavascriptPath is the path for the embedded JaWS Javascript library.
var JavascriptPath = makeJavascriptPath()

func makeJavascriptGZip() []byte {
	b := bytes.Buffer{}
	gw := gzip.NewWriter(&b)
	_, err := gw.Write(JavascriptText)
	maybePanic(err)
	maybePanic(gw.Close())
	return b.Bytes()
}

func makeJavascriptPath() string {
	h := fnv.New64a()
	_, err := h.Write(JavascriptText)
	maybePanic(err)
	return "/jaws/jaws." + strconv.FormatUint(h.Sum64(), 36) + ".js"
}

func JawsKeyString(jawsKey uint64) string {
	if jawsKey == 0 {
		return ""
	}
	return strconv.FormatUint(jawsKey, 16)
}

// HeadHTML returns HTML code to load the JaWS javascript along with other scripts and CSS files given.
// Normally you would use Request.HeadHTML or Jaws.HeadHTML instead of calling this function directly.
// Place the returned HTML code in the HEAD section of the document.
func HeadHTML(jawsKey uint64, js []string, css []string) template.HTML {
	const forEachPart = `.forEach(function(c){var e=document.createElement("script");e.src=c;e.async=!1;document.head.appendChild(e);});`
	keyStr := JawsKeyString(jawsKey)

	need := 21 + len(keyStr) + 4 + 3 + len(JavascriptPath) + 2 + len(forEachPart) + 9 + 18
	for _, e := range js {
		need += 3 + len(e)
	}
	for _, e := range css {
		need += 29 + len(e) + 2
	}

	s := make([]byte, 0, need)
	s = append(s, `<script>var jawsKey="`...)
	s = append(s, keyStr...)
	s = append(s, `";["`...)
	s = append(s, JavascriptPath...)
	for _, e := range js {
		s = append(s, `","`...)
		s = append(s, e...)
	}
	s = append(s, `"]`+forEachPart+`</script>`...)

	for _, e := range css {
		s = append(s, `<link rel="stylesheet" href="`...)
		s = append(s, e...)
		s = append(s, `">`...)
	}

	s = append(s, `<script>0</script>`...) // forces FireFox to evaluate now
	return template.HTML(s)                // #nosec G203
}
