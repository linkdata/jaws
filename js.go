package jaws

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"hash/fnv"
	"strconv"
	"strings"
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
	return "/jaws/.jaws." + strconv.FormatUint(h.Sum64(), 36) + ".js"
}

// JawsKeyAppend appends the JaWS key as a string to the buffer.
func JawsKeyAppend(b []byte, jawsKey uint64) []byte {
	if jawsKey != 0 {
		b = strconv.AppendUint(b, jawsKey, 32)
	}
	return b
}

// JawsKeyString returns the string to be used for the given JaWS key.
func JawsKeyString(jawsKey uint64) string {
	return string(JawsKeyAppend(nil, jawsKey))
}

// JawsKeyValue parses a key string (as returned JawsKeyString) into a uint64.
func JawsKeyValue(jawsKey string) uint64 {
	slashIdx := strings.IndexByte(jawsKey, '/')
	if slashIdx < 0 {
		slashIdx = len(jawsKey)
	}
	if val, err := strconv.ParseUint(jawsKey[:slashIdx], 32, 64); err == nil {
		return val
	}
	return 0
}

const jsLoader = `.forEach(function(c){var e=document.createElement("script");e.src=c;e.async=!1;document.head.appendChild(e);});`

// HeadHTML returns HTML code to load the given scripts and CSS files efficiently.
func HeadHTML(js []string, css []string) string {
	var s []byte

	for _, e := range css {
		s = append(s, "<link rel=\"preload\" href="...)
		s = strconv.AppendQuote(s, e)
		s = append(s, " as=\"style\">\n"...)
	}
	for _, e := range js {
		s = append(s, "<link rel=\"preload\" href="...)
		s = strconv.AppendQuote(s, e)
		s = append(s, " as=\"script\">\n"...)
	}
	s = append(s, "<script>["...)
	for i, e := range js {
		if i > 0 {
			s = append(s, ',')
		}
		s = strconv.AppendQuote(s, e)
	}
	s = append(s, "]"+jsLoader+"</script>\n"...)

	for _, e := range css {
		s = append(s, "<link rel=\"stylesheet\" href="...)
		s = strconv.AppendQuote(s, e)
		s = append(s, ">\n"...)
	}

	return string(s)
}
