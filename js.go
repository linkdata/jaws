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

const bootstrapVersion = "5.1.3"

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

const bootstrapCDN = "https://cdn.jsdelivr.net/npm/bootstrap"
const bootstrapCSS = bootstrapCDN + "@" + bootstrapVersion + "/dist/css/bootstrap.min.css"
const bootstrapJS = bootstrapCDN + "@" + bootstrapVersion + "/dist/js/bootstrap.bundle.min.js"

// HeadHTML returns the HTML code to load the required CSS and Javascript
// libraries along with any `*.js“ and `*.css` URL's given in `extra`.
// Place the returned HTML code in the HEAD section of the document.
func HeadHTML(jawsKey uint64, extra ...string) template.HTML {
	const forEachPart = `.forEach(function(c){var e=document.createElement("script");e.src=c;e.async=!1;document.head.appendChild(e);});`

	s := make([]byte, 0, 512)
	s = append(s, `<script>var jawsKey="`...)
	s = append(s, JawsKeyString(jawsKey)...)
	s = append(s, `";["`...)
	s = append(s, bootstrapJS...)
	s = append(s, `","`...)
	s = append(s, JavascriptPath()...)
	for _, script := range extra {
		if strings.HasSuffix(script, ".js") {
			s = append(s, `","`...)
			s = append(s, script...)
		}
	}
	s = append(s, `"]`+forEachPart+`</script>`...)
	s = appendLinkStyleSheet(s, bootstrapCSS)
	for _, script := range extra {
		if strings.HasSuffix(script, ".css") {
			s = appendLinkStyleSheet(s, script)
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
