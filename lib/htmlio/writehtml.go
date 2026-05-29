package htmlio

import (
	"html"
	"html/template"
	"io"

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

// AppendAttrValue appends value as a double-quoted HTML attribute value.
//
// The value parameter must be the unescaped logical attribute value. It is
// escaped for HTML source output by this function.
func AppendAttrValue(b []byte, value string) []byte {
	b = append(b, '"')
	b = append(b, html.EscapeString(value)...)
	b = append(b, '"')
	return b
}

// AppendAttr appends a trusted attribute name with an escaped value.
//
// The name parameter must be a trusted attribute name. The value parameter must
// be the unescaped logical attribute value; it is escaped for HTML source output
// by this function.
func AppendAttr(b []byte, name, value string) []byte {
	b = append(b, ' ')
	b = append(b, name...)
	b = append(b, '=')
	b = AppendAttrValue(b, value)
	return b
}

// Attr returns a trusted attribute name with an escaped value.
//
// The name parameter must be a trusted attribute name. The value parameter must
// be the unescaped logical attribute value; it is escaped for HTML source output
// by this function.
func Attr(name, value string) template.HTMLAttr {
	// AppendAttr writes a leading space (it is meant for joining onto a tag);
	// [1:] drops that space since Attr returns the attribute on its own.
	return template.HTMLAttr(AppendAttr(nil, name, value)[1:]) // #nosec G203
}

func appendHTMLTag(b []byte, jid jid.Jid, htmlTag, typeAttr, valueAttr string, attrs []template.HTMLAttr) []byte {
	b = jid.AppendStartTagAttr(b, htmlTag)
	if typeAttr != "" {
		b = AppendAttr(b, "type", typeAttr)
	}
	if valueAttr != "" {
		b = AppendAttr(b, "value", valueAttr)
	}
	b = appendAttrs(b, attrs)
	b = append(b, '>')
	return b
}

// WriteHTMLTag writes an HTML start tag with optional id, type, value and raw
// attribute fragments.
//
// The typeAttr and valueAttr parameters must be unescaped logical values; they
// are escaped for HTML source output. The attrs parameter contains trusted raw
// attribute fragments and is written as-is.
func WriteHTMLTag(w io.Writer, jid jid.Jid, htmlTag, typeAttr, valueAttr string, attrs []template.HTMLAttr) (err error) {
	var buf [htmlBufferSize]byte
	b := appendHTMLTag(buf[:0], jid, htmlTag, typeAttr, valueAttr, attrs)
	_, err = w.Write(b)
	return
}

// WriteHTMLInput writes an input start tag with optional id, type, value and
// raw attribute fragments.
//
// The typeAttr and valueAttr parameters must be unescaped logical values; they
// are escaped for HTML source output. The attrs parameter contains trusted raw
// attribute fragments and is written as-is.
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
