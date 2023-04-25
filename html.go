package jaws

import (
	"html/template"
)

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

func HtmlInput(id, typ, val string, attrs ...string) template.HTML {
	need := 11 + len(id) + 8 + len(typ) + 9 + len(val) + 1 + 1 + getAttrsLen(attrs) + 1
	b := make([]byte, 0, need)
	b = append(b, `<input jid="`...)
	b = append(b, id...)
	b = append(b, `" type="`...)
	b = append(b, typ...)
	if val != "" {
		b = append(b, `" value="`...)
		b = append(b, val...)
	}
	b = append(b, '"')
	b = appendAttrs(b, attrs)
	b = append(b, '>')
	return template.HTML(b) // #nosec G203
}

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

func HtmlInner(id, tag, typ, inner string, attrs ...string) template.HTML {
	need := 1 + len(tag)*2 + 5 + len(id) + 8 + len(typ) + 1 + 1 + getAttrsLen(attrs) + 1 + len(inner) + 2 + 1
	b := make([]byte, 0, need)
	b = append(b, '<')
	b = append(b, tag...)
	b = append(b, ` jid="`...)
	b = append(b, id...)
	if typ != "" {
		b = append(b, `" type="`...)
		b = append(b, typ...)
	}
	b = append(b, '"')
	b = appendAttrs(b, attrs)
	b = append(b, '>')
	if inner != "" || needClosingTag(tag) {
		b = append(b, inner...)
		b = append(b, "</"...)
		b = append(b, tag...)
		b = append(b, '>')
	}
	return template.HTML(b) // #nosec G203
}

func HtmlSelect(jid string, val *NamedBools, attrs ...string) template.HTML {
	need := 12 + len(jid) + 2 + getAttrsLen(attrs) + 2 + 10
	val.ReadLocked(func(nba []*NamedBool) {
		for _, nb := range nba {
			need += 15 + len(nb.Name) + 2 + len(nb.Text) + 10
			if nb.Checked {
				need += 9
			}
		}
	})
	b := make([]byte, 0, need)
	b = append(b, `<select jid="`...)
	b = append(b, jid...)
	b = append(b, '"')
	b = appendAttrs(b, attrs)
	b = append(b, ">\n"...)
	val.ReadLocked(func(nba []*NamedBool) {
		for _, nb := range nba {
			b = append(b, `<option value="`...)
			b = append(b, nb.Name...)
			if nb.Checked {
				b = append(b, `" selected>`...)
			} else {
				b = append(b, `">`...)
			}
			b = append(b, nb.Text...)
			b = append(b, "</option>\n"...)
		}
	})
	b = append(b, "</select>\n"...)
	return template.HTML(b) // #nosec G203
}
