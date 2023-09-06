package jaws

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"strings"
)

type Template struct {
	*template.Template
	Dot interface{}
}

func (t *Template) String() string {
	return fmt.Sprintf("Template{%q, %v}", t.Template.Name(), t.Dot)
}

func (t Template) JawsGet(e *Element) interface{} {
	var b bytes.Buffer
	if err := e.Jaws.Log(t.Execute(&b, t.Dot)); err != nil {
		msg := fmt.Sprintf("<!-- %v: %s: %v", e, t.String(), err)
		b.Write([]byte(strings.ReplaceAll(html.EscapeString(msg), "-->", "")))
		b.Write([]byte(" -->"))
	}
	return template.HTML(b.String())
}

func (t Template) JawsSet(e *Element, val interface{}) bool {
	panic(fmt.Sprintf("jaws: Element %v: ValueProxy for %T is read-only", e, val))
}
