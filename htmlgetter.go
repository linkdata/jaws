package jaws

import (
	"html/template"
)

type HtmlGetter interface {
	JawsGetHtml(e *Element) template.HTML
}
