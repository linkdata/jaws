package jaws

import (
	"bytes"
	"fmt"
	"html/template"
	"io"

	"github.com/linkdata/jaws/what"
)

type Template struct {
	Dot interface{}
	*template.Template
}

// GetTemplate resolves 'v' to a *template.Template or panics.
func (rq *Request) MustTemplate(v interface{}) (tp *template.Template) {
	switch v := v.(type) {
	case *template.Template:
		tp = v
	case string:
		tp = rq.Jaws.Template.Lookup(v)
	}
	if tp == nil {
		panic(fmt.Errorf("expected template, not %v", v))
	}
	return
}

func (rq *Request) MakeTemplate(templ, dot interface{}) Template {
	return Template{Template: rq.MustTemplate(templ), Dot: dot}
}

func (t Template) String() string {
	return fmt.Sprintf("{%q, %s}", t.Template.Name(), TagString(t.Dot))
}

func (t Template) ToHTML(e *Element) template.HTML {
	var b bytes.Buffer
	t.JawsRender(e, &b, nil)
	return template.HTML(b.String())
}

var _ UI = (*Template)(nil) // statically ensure interface is defined

func (t Template) JawsRender(e *Element, w io.Writer, params []interface{}) {
	writeUiDebug(e, w)
	maybePanic(t.Execute(w, With{Element: e, Dot: t.Dot}))
}

func (t Template) JawsUpdate(u *Element) {
	u.Replace(t.ToHTML(u))
}

var _ EventHandler = (*Template)(nil) // statically ensure interface is defined

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

var _ HtmlGetter = (*Template)(nil) // statically ensure interface is defined

func (t Template) JawsGetHtml(e *Element) template.HTML {
	return t.ToHTML(e)
}
