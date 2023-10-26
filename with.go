package jaws

import (
	"html/template"
)

type With struct {
	*Element
	RequestWriter
	Dot   interface{}
	Attrs template.HTMLAttr
}
