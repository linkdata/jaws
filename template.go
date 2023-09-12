package jaws

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/linkdata/jaws/what"
)

type Template struct {
	*template.Template
	Dot interface{}
}

func (rq *Request) NewTemplate(templ, dot interface{}) Template {
	var tp *template.Template
	if name, ok := templ.(string); ok {
		tp = rq.Jaws.Template.Lookup(name)
	} else {
		tp = templ.(*template.Template)
	}
	return Template{Template: tp, Dot: dot}
}

func (t *Template) String() string {
	return fmt.Sprintf("Template{%q, %v}", t.Template.Name(), t.Dot)
}

func (t *Template) ToHTML(e *Element) (template.HTML, error) {
	var b bytes.Buffer
	err := t.JawsRender(e, &b)
	if err != nil {
		b.Reset()
		msg := fmt.Sprintf("<!-- %v: %s: %v", e, t.String(), err)
		b.Write([]byte(strings.ReplaceAll(msg, "--", "==")))
		b.Write([]byte(" -->"))
	}
	return template.HTML(b.String()), err
}

func (t Template) JawsTags(rq *Request, inTags []interface{}) []interface{} {
	return append(inTags, t.Dot, t.Template)
}

func (t Template) JawsRender(e *Element, w io.Writer) error {
	writeUiDebug(e, w)
	return t.Execute(w, With{Element: e, Dot: t.Dot})
}

func (t Template) JawsUpdate(e *Element, u Updater) error {
	h, err := t.ToHTML(e)
	if err == nil {
		u.Replace(h)
	}
	return err
}

func (t Template) JawsEvent(e *Element, wht what.What, val string) error {
	if wht == what.Click {
		if h, ok := t.Dot.(ClickHandler); ok {
			return h.JawsClick(e, val)
		}
	}
	if h, ok := t.Dot.(EventHandler); ok {
		return h.JawsEvent(e, wht, val)
	}
	return nil
}

func (t Template) JawsGet(e *Element) interface{} {
	if h, err := t.ToHTML(e); err == nil {
		return h
	}
	return ""
}

func (t Template) JawsSet(e *Element, val interface{}) bool {
	panic("jaws: Template.JawsSet(): read-only")
}
