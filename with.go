package jaws

import (
	"html/template"
)

type With struct {
	*Element
	Dot   interface{}
	Attrs template.HTMLAttr
}
