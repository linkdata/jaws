package jaws

import (
	"html/template"
)

type HTMLGetter interface {
	JawsGetHTML(e *Element) template.HTML
}
