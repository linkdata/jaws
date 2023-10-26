package jaws

import (
	"html/template"
)

type With struct {
	ElementWriter
	Dot   interface{}
	Attrs template.HTMLAttr
}
