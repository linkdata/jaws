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
	b = append(b, `<input id="`...)
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

func HtmlInner(id, tag, typ, inner string, attrs ...string) template.HTML {
	need := 1 + len(tag)*2 + 5 + len(id) + 8 + len(typ) + 1 + 1 + getAttrsLen(attrs) + 1 + len(inner) + 2 + 1
	b := make([]byte, 0, need)
	b = append(b, '<')
	b = append(b, tag...)
	b = append(b, ` id="`...)
	b = append(b, id...)
	if typ != "" {
		b = append(b, `" type="`...)
		b = append(b, typ...)
	}
	b = append(b, '"')
	b = appendAttrs(b, attrs)
	b = append(b, '>')
	b = append(b, inner...)
	b = append(b, "</"...)
	b = append(b, tag...)
	b = append(b, '>')
	return template.HTML(b) // #nosec G203
}

func HtmlSelect(id string, val *NamedBoolArray, attrs ...string) template.HTML {
	need := 12 + len(id) + 2 + getAttrsLen(attrs) + 2 + 10
	if val != nil {
		for _, nb := range *val {
			need += 15 + len(nb.Value) + 2 + len(nb.Text) + 10
			if nb.Checked {
				need += 9
			}
		}
	}
	b := make([]byte, 0, need)
	b = append(b, `<select id="`...)
	b = append(b, id...)
	b = append(b, '"')
	b = appendAttrs(b, attrs)
	b = append(b, ">\n"...)
	if val != nil {
		for _, nb := range *val {
			b = append(b, `<option value="`...)
			b = append(b, nb.Value...)
			if nb.Checked {
				b = append(b, `" selected>`...)
			} else {
				b = append(b, `">`...)
			}
			b = append(b, nb.Text...)
			b = append(b, "</option>\n"...)
		}
	}
	b = append(b, "</select>\n"...)
	return template.HTML(b) // #nosec G203
}
