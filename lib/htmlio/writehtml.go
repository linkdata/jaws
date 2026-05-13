package htmlio

import (
	"html/template"
	"io"
	"strconv"

	"github.com/linkdata/jaws/lib/jid"
)

var singletonTags = map[string]struct{}{
	"area":    {},
	"base":    {},
	"br":      {},
	"col":     {},
	"command": {},
	"embed":   {},
	"hr":      {},
	"img":     {},
	"input":   {},
	"keygen":  {},
	"link":    {},
	"meta":    {},
	"param":   {},
	"source":  {},
	"track":   {},
	"wbr":     {},
}

const htmlBufferSize = 128

func needClosingTag(tag string) bool {
	_, ok := singletonTags[tag]
	return !ok
}

func appendAttrs(b []byte, attrs []template.HTMLAttr) []byte {
	for _, s := range attrs {
		if s != "" {
			b = append(b, ' ')
			b = append(b, s...)
		}
	}
	return b
}

func appendHTMLTag(b []byte, jid jid.Jid, htmlTag, typeAttr, valueAttr string, attrs []template.HTMLAttr) []byte {
	b = jid.AppendStartTagAttr(b, htmlTag)
	if typeAttr != "" {
		b = append(b, ` type=`...)
		b = strconv.AppendQuote(b, typeAttr)
	}
	if valueAttr != "" {
		b = append(b, ` value=`...)
		b = strconv.AppendQuote(b, valueAttr)
	}
	b = appendAttrs(b, attrs)
	b = append(b, '>')
	return b
}

// WriteHTMLTag writes an HTML start tag with optional id, type, value and raw
// attribute fragments.
func WriteHTMLTag(w io.Writer, jid jid.Jid, htmlTag, typeAttr, valueAttr string, attrs []template.HTMLAttr) (err error) {
	var buf [htmlBufferSize]byte
	b := appendHTMLTag(buf[:0], jid, htmlTag, typeAttr, valueAttr, attrs)
	_, err = w.Write(b)
	return
}

// WriteHTMLInput writes an input start tag with optional id, type, value and
// raw attribute fragments.
func WriteHTMLInput(w io.Writer, jid jid.Jid, typeAttr, valueAttr string, attrs []template.HTMLAttr) (err error) {
	return WriteHTMLTag(w, jid, "input", typeAttr, valueAttr, attrs)
}

// WriteHTMLInner writes an HTML element with trusted inner HTML.
//
// Singleton tags such as img and input are written without closing tags unless
// innerHTML is non-empty.
func WriteHTMLInner(w io.Writer, jid jid.Jid, htmlTag, typeAttr string, innerHTML template.HTML, attrs ...template.HTMLAttr) (err error) {
	var buf [htmlBufferSize]byte
	b := appendHTMLTag(buf[:0], jid, htmlTag, typeAttr, "", attrs)
	if innerHTML != "" || needClosingTag(htmlTag) {
		b = append(b, innerHTML...)
		b = append(b, "</"...)
		b = append(b, htmlTag...)
		b = append(b, '>')
	}
	_, err = w.Write(b)
	return
}
