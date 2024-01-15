package jaws

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

const jidPrealloc = 7 + 4

func needClosingTag(tag string) bool {
	_, ok := singletonTags[tag]
	return !ok
}

func getAttrsLen(attrs []template.HTMLAttr) (attrslen int) {
	for _, s := range attrs {
		if s != "" {
			attrslen += 1 + len(s)
		}
	}
	return
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

func WriteHtmlTag(w io.Writer, jid jid.Jid, htmlTag, typeAttr, valueAttr string, attrs []template.HTMLAttr) (err error) {
	b := make([]byte, 0, 64)
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
	_, err = w.Write(b)
	return
}

func WriteHtmlInput(w io.Writer, jid jid.Jid, typeAttr, valueAttr string, attrs []template.HTMLAttr) (err error) {
	return WriteHtmlTag(w, jid, "input", typeAttr, valueAttr, attrs)
}

func WriteHtmlInner(w io.Writer, jid jid.Jid, htmlTag, typeAttr string, innerHtml template.HTML, attrs ...template.HTMLAttr) (err error) {
	if err = WriteHtmlTag(w, jid, htmlTag, typeAttr, "", attrs); err == nil {
		if innerHtml != "" || needClosingTag(htmlTag) {
			w.Write([]byte(innerHtml))
			var b []byte
			b = append(b, "</"...)
			b = append(b, htmlTag...)
			b = append(b, '>')
			w.Write(b)
		}
	}
	return
}

func WriteHtmlSelect(w io.Writer, jid jid.Jid, nba *NamedBoolArray, attrs ...template.HTMLAttr) (err error) {
	need := 12 + jidPrealloc + 2 + getAttrsLen(attrs) + 2 + 10
	nba.ReadLocked(func(nba []*NamedBool) {
		for _, nb := range nba {
			need += 15 + len(nb.Name()) + 2 + len(nb.Html()) + 10
			if nb.Checked() {
				need += 9
			}
		}
	})
	b := make([]byte, 0, need)
	b = jid.AppendStartTagAttr(b, "select")
	b = appendAttrs(b, attrs)
	b = append(b, ">\n"...)
	nba.ReadLocked(func(nba []*NamedBool) {
		for _, nb := range nba {
			b = append(b, `<option value=`...)
			b = strconv.AppendQuote(b, nb.Name())
			if nb.Checked() {
				b = append(b, ` selected`...)
			}
			b = append(b, '>')
			b = append(b, nb.Html()...)
			b = append(b, "</option>\n"...)
		}
	})
	b = append(b, "</select>\n"...)
	_, err = w.Write(b)
	return
}
