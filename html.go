package jaws

import (
	"html/template"
	"io"
	"strconv"
	"strings"
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

func getAttrsLen(attrs []string) (attrslen int) {
	for _, s := range attrs {
		if s != "" {
			attrslen += 1 + len(s)
		}
	}
	return
}

func appendAttrs(b []byte, attrs []string) []byte {
	for _, s := range attrs {
		if s != "" {
			b = append(b, ' ')
			b = append(b, s...)
		}
	}
	return b
}

func WriteHtmlInput(w io.Writer, jid Jid, typ, val string, attrs ...string) (err error) {
	need := 11 + jidPrealloc + 8 + len(typ) + 9 + len(val) + 1 + 1 + getAttrsLen(attrs) + 1
	b := make([]byte, 0, need)
	b = jid.AppendStartTagAttr(b, "input")
	b = append(b, ` type=`...)
	b = strconv.AppendQuote(b, typ)
	if val != "" {
		b = append(b, ` value=`...)
		b = strconv.AppendQuote(b, val)
	}
	b = appendAttrs(b, attrs)
	b = append(b, '>')
	_, err = w.Write(b)
	return
}

func HtmlInput(jid Jid, typ, val string, attrs ...string) template.HTML {
	var sb strings.Builder
	_ = WriteHtmlInput(&sb, jid, typ, val, attrs...)
	return template.HTML(sb.String()) // #nosec G203
}

func WriteHtmlInner(w io.Writer, jid Jid, tag, typ string, inner template.HTML, attrs ...string) (err error) {
	need := 1 + len(tag)*2 + jidPrealloc + 8 + len(typ) + 1 + 1 + getAttrsLen(attrs) + 1 + len(inner) + 2 + 1
	b := make([]byte, 0, need)
	b = jid.AppendStartTagAttr(b, tag)
	if typ != "" {
		b = append(b, ` type=`...)
		b = strconv.AppendQuote(b, typ)
	}
	b = appendAttrs(b, attrs)
	b = append(b, '>')
	if inner != "" || needClosingTag(tag) {
		b = append(b, inner...)
		b = append(b, "</"...)
		b = append(b, tag...)
		b = append(b, '>')
	}
	_, err = w.Write(b)
	return
}

func HtmlInner(jid Jid, tag, typ string, inner template.HTML, attrs ...string) template.HTML {
	var sb strings.Builder
	_ = WriteHtmlInner(&sb, jid, tag, typ, inner, attrs...)
	return template.HTML(sb.String()) // #nosec G203
}

func WriteHtmlSelect(w io.Writer, jid Jid, nba *NamedBoolArray, attrs ...string) (err error) {
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

func HtmlSelect(jid Jid, nba *NamedBoolArray, attrs ...string) template.HTML {
	var sb strings.Builder
	_ = WriteHtmlSelect(&sb, jid, nba, attrs...)
	return template.HTML(sb.String()) // #nosec G203
}
