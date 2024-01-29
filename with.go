package jaws

import (
	"html/template"
)

type With struct {
	*Element
	RequestWriter
	Dot   any
	Attrs template.HTMLAttr
}
