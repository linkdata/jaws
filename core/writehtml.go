package core

import (
	"html/template"
	"io"
	"strconv"

	"github.com/linkdata/jaws/jid"
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

func WriteHTMLTag(w io.Writer, jid jid.Jid, htmlTag, typeAttr, valueAttr string, attrs []template.HTMLAttr) (err error) {
	b := jid.AppendStartTagAttr(nil, htmlTag)
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
	_, err = w.Write(b)
	return
}

func WriteHTMLInput(w io.Writer, jid jid.Jid, typeAttr, valueAttr string, attrs []template.HTMLAttr) (err error) {
	return WriteHTMLTag(w, jid, "input", typeAttr, valueAttr, attrs)
}

func WriteHTMLInner(w io.Writer, jid jid.Jid, htmlTag, typeAttr string, innerHTML template.HTML, attrs ...template.HTMLAttr) (err error) {
	if err = WriteHTMLTag(w, jid, htmlTag, typeAttr, "", attrs); err == nil {
		if innerHTML != "" || needClosingTag(htmlTag) {
			var b []byte
			b = append(b, innerHTML...)
			b = append(b, "</"...)
			b = append(b, htmlTag...)
			b = append(b, '>')
			_, err = w.Write(b)
		}
	}
	return
}

func WriteHTMLSelect(w io.Writer, jid jid.Jid, nba *NamedBoolArray, attrs []template.HTMLAttr) (err error) {
	if err = WriteHTMLTag(w, jid, "select", "", "", attrs); err == nil {
		var b []byte
		nba.ReadLocked(func(nba []*NamedBool) {
			for _, nb := range nba {
				b = append(b, "\n<option value="...)
				b = strconv.AppendQuote(b, nb.Name())
				if nb.Checked() {
					b = append(b, ` selected`...)
				}
				b = append(b, '>')
				b = append(b, nb.HTML()...)
				b = append(b, "</option>"...)
			}
		})
		b = append(b, "\n</select>"...)
		_, err = w.Write(b)
	}
	return
}
