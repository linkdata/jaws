package htmlio

import (
	"html"
	"html/template"
	"io"
	"strings"

	"github.com/linkdata/jaws/lib/jid"
)

// voidElements are the HTML void elements, which take no closing tag and cannot
// contain content.
var voidElements = map[string]struct{}{
	"area":   {},
	"base":   {},
	"br":     {},
	"col":    {},
	"embed":  {},
	"hr":     {},
	"img":    {},
	"input":  {},
	"link":   {},
	"meta":   {},
	"param":  {},
	"source": {},
	"track":  {},
	"wbr":    {},
}

// needClosingTag reports whether tag is not an HTML void element (case-insensitive),
// and so takes a closing tag.
func needClosingTag(tag string) bool {
	_, ok := voidElements[strings.ToLower(tag)]
	return !ok
}

// isNewlineSensitive reports whether tag (case-insensitive) is a textarea or pre
// element, for which the HTML parser strips one newline immediately following the
// start tag.
func isNewlineSensitive(tag string) bool {
	switch strings.ToLower(tag) {
	case "textarea", "pre":
		return true
	}
	return false
}

// appendEscapeCR appends s to b, replacing every carriage return with the
// numeric character reference &#13;.
//
// Browser input-stream preprocessing rewrites a raw CR (and a CRLF pair) to a
// single LF before tokenization, so a raw carriage return never reaches the DOM.
// The reference is decoded back to CR after preprocessing, so encoding it lets
// logical values that contain carriage returns round-trip through HTML parsing.
func appendEscapeCR(b []byte, s string) []byte {
	for {
		i := strings.IndexByte(s, '\r')
		if i < 0 {
			return append(b, s...)
		}
		b = append(b, s[:i]...)
		b = append(b, "&#13;"...)
		s = s[i+1:]
	}
}

// AppendAttrs appends each non-empty attribute fragment in attrs to b, each
// prefixed with a single space so the result can be concatenated directly after a
// tag name.
//
// The attrs are trusted raw HTML attribute fragments written verbatim with no
// escaping; they MUST NOT contain untrusted data. Use [Attr] or [AppendAttr] to
// build attribute fragments with an escaped value.
func AppendAttrs(b []byte, attrs []template.HTMLAttr) []byte {
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
// escaped for HTML source output by this function. Use [Attr] or [AppendAttr]
// to build a complete name=value fragment.
//
// Carriage returns are emitted as the numeric character reference &#13;, because
// browser input-stream preprocessing would otherwise rewrite a raw CR to LF
// before parsing and change the value the DOM reports.
func AppendAttrValue(b []byte, value string) []byte {
	b = append(b, '"')
	b = appendEscapeCR(b, html.EscapeString(value))
	b = append(b, '"')
	return b
}

// AppendAttr appends a trusted attribute name with an escaped value.
//
// The name parameter is written verbatim with no escaping or validation and
// MUST NOT be derived from untrusted data, or it becomes an HTML-injection
// primitive. The value parameter must be the unescaped logical attribute value;
// it is escaped for HTML source output by this function.
func AppendAttr(b []byte, name, value string) []byte {
	b = append(b, ' ')
	b = append(b, name...)
	b = append(b, '=')
	b = AppendAttrValue(b, value)
	return b
}

// Attr returns a trusted attribute name with an escaped value.
//
// The name parameter is written verbatim with no escaping or validation and
// MUST NOT be derived from untrusted data, or it becomes an HTML-injection
// primitive. The value parameter must be the unescaped logical attribute value;
// it is escaped for HTML source output by this function.
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
	b = AppendAttrs(b, attrs)
	b = append(b, '>')
	return b
}

// WriteHTMLTag writes an HTML start tag with optional id, type, value and raw
// attribute fragments.
//
// The htmlTag parameter is trusted and written verbatim with no escaping or
// validation; it MUST NOT be derived from untrusted data. The typeAttr and
// valueAttr parameters must be unescaped logical values; they are escaped for
// HTML source output. The attrs parameter contains trusted raw attribute
// fragments and is written verbatim with no escaping; it MUST NOT contain
// untrusted data. Use [Attr] or [AppendAttr] to build attribute fragments with
// an escaped value.
func WriteHTMLTag(w io.Writer, jid jid.Jid, htmlTag, typeAttr, valueAttr string, attrs []template.HTMLAttr) (err error) {
	b := appendHTMLTag(nil, jid, htmlTag, typeAttr, valueAttr, attrs)
	_, err = w.Write(b)
	return
}

// WriteHTMLInput writes an input start tag with optional id, type, value and
// raw attribute fragments. The id attribute is emitted only for a positive
// [jid.Jid].
//
// The typeAttr and valueAttr parameters must be unescaped logical values; they
// are escaped for HTML source output. The attrs parameter contains trusted raw
// attribute fragments and is written verbatim with no escaping; it MUST NOT
// contain untrusted data, nor must typeAttr be derived from untrusted data. Use
// [Attr] or [AppendAttr] to build attribute fragments with an escaped value.
func WriteHTMLInput(w io.Writer, jid jid.Jid, typeAttr, valueAttr string, attrs []template.HTMLAttr) (err error) {
	return WriteHTMLTag(w, jid, "input", typeAttr, valueAttr, attrs)
}

// WriteHTMLInner writes an HTML element with trusted inner HTML.
//
// Void elements such as img and input are written without a closing tag, and any
// innerHTML passed for them is ignored, since a void element cannot contain
// content (emitting "<img>...</img>" would be invalid HTML).
//
// Unlike [WriteHTMLTag] it emits no value attribute; pass one via attrs (for
// example Attr("value", v)) when a value="..." is needed.
//
// For textarea and pre elements, the HTML source includes one LF immediately
// after the start tag. The parser strips that one LF, so a leading LF in
// innerHTML is preserved instead of being consumed.
//
// Carriage returns in innerHTML are written verbatim, not encoded. A textarea
// reports its value with every CR and CRLF normalized to LF (the HTML standard's
// textarea value normalization), so carriage returns in textarea content cannot
// round-trip through the value the browser sends back. Encoding them would also
// be unsafe for pre, whose trusted markup may include script, comment, or other
// contexts where character references are not decoded. Use [AppendAttrValue] for
// logical values that must retain carriage returns.
//
// The htmlTag parameter is trusted and written verbatim with no escaping or
// validation; it MUST NOT be derived from untrusted data. The typeAttr parameter
// must be an unescaped logical value; it is escaped for HTML source output. The
// attrs parameter contains trusted raw attribute fragments and is written
// verbatim with no escaping; it MUST NOT contain untrusted data. Use [Attr] or
// [AppendAttr] to build attribute fragments with an escaped value.
func WriteHTMLInner(w io.Writer, jid jid.Jid, htmlTag, typeAttr string, innerHTML template.HTML, attrs ...template.HTMLAttr) (err error) {
	b := appendHTMLTag(nil, jid, htmlTag, typeAttr, "", attrs)
	if needClosingTag(htmlTag) {
		// The HTML parser strips one LF right after a textarea/pre start tag.
		// Provide that parser-consumed prefix so a leading LF in innerHTML is
		// preserved instead of being consumed. Carriage returns are written
		// verbatim; the doc comment explains why they are not encoded here.
		if isNewlineSensitive(htmlTag) {
			b = append(b, '\n')
		}
		b = append(b, innerHTML...)
		b = append(b, "</"...)
		b = append(b, htmlTag...)
		b = append(b, '>')
	}
	_, err = w.Write(b)
	return
}
