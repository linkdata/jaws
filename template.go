package jaws

import (
	"bytes"
	"fmt"
	"html/template"
)

type Template struct {
	Dot interface{}
	*template.Template
}

func (t Template) JawsGet(e *Element) interface{} {
	var b bytes.Buffer
	e.Jaws.MustLog(t.Execute(&b, e.With(t.Dot)))
	return template.HTML(b.String())
}

func (t Template) JawsSet(e *Element, val interface{}) bool {
	panic(fmt.Sprintf("jaws: Element %v: ValueProxy for %T is read-only", e, val))
}
