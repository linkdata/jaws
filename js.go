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

// HeadHTML returns HTML code to load the given scripts and CSS files efficiently.
func HeadHTML(js []string, css []string) template.HTML {
	var s []byte

	for _, e := range css {
		s = append(s, "<link rel=\"preload\" href=\""...)
		s = append(s, e...)
		s = append(s, "\" as=\"style\">\n"...)
	}
	for _, e := range js {
		s = append(s, "<link rel=\"preload\" href=\""...)
		s = append(s, e...)
		s = append(s, "\" as=\"script\">\n"...)
	}

	s = append(s, "<script>["...)
	for i, e := range js {
		if i > 0 {
			s = append(s, ',')
		}
		s = append(s, '"')
		s = append(s, e...)
		s = append(s, '"')
	}
	s = append(s, "].forEach(function(c){var e=document.createElement(\"script\");e.src=c;e.async=!1;document.head.appendChild(e);});</script>\n"...)

	for _, e := range css {
		s = append(s, "<link rel=\"stylesheet\" href=\""...)
		s = append(s, e...)
		s = append(s, "\">\n"...)
	}

	return template.HTML(s) // #nosec G203
}
