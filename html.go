package jaws

import (
	"html/template"
)

func HtmlInput(id, typ, val, attrs string) template.HTML {
	need := 11 + len(id) + 8 + len(typ) + 9 + len(val) + 1 + 1 + len(attrs) + 1
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
	if attrs != "" {
		b = append(b, ' ')
		b = append(b, attrs...)
	}
	b = append(b, '>')
	return template.HTML(b) // #nosec G203
}

func HtmlInner(id, tag, typ, inner, attrs string) template.HTML {
	need := 1 + len(tag)*2 + 5 + len(id) + 8 + len(typ) + 1 + 1 + len(attrs) + 1 + len(inner) + 2 + 1
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
	if attrs != "" {
		b = append(b, ' ')
		b = append(b, attrs...)
	}
	b = append(b, '>')
	b = append(b, inner...)
	b = append(b, "</"...)
	b = append(b, tag...)
	b = append(b, '>')
	return template.HTML(b) // #nosec G203
}

func HtmlSelect(id string, val *NamedBoolArray, attrs string) template.HTML {
	need := 12 + len(id) + 2 + len(attrs) + 2 + 10
	for _, nb := range *val {
		need += 15 + len(nb.Value) + 2 + len(nb.Text) + 10
		if nb.Checked {
			need += 9
		}
	}
	b := make([]byte, 0, need)
	b = append(b, `<select id="`...)
	b = append(b, id...)
	b = append(b, '"')
	if attrs != "" {
		b = append(b, ' ')
		b = append(b, attrs...)
	}
	b = append(b, ">\n"...)
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
	b = append(b, "</select>\n"...)
	return template.HTML(b) // #nosec G203
}
