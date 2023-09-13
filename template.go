package jaws

import (
	"bytes"
	"fmt"
	"html/template"
	"io"

	"github.com/linkdata/jaws/what"
)

type Template struct {
	Container
	Dot interface{}
	*template.Template
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

func (t Template) String() string {
	return fmt.Sprintf("{%s, %q, %s}", TagString(t.Container), t.Template.Name(), TagString(t.Dot))
}

func (t Template) ToHTML(e *Element) template.HTML {
	var b bytes.Buffer
	t.JawsRender(e, &b)
	return template.HTML(b.String())
}

func (t Template) JawsTags(rq *Request, inTags []interface{}) []interface{} {
	if tagger, ok := t.Dot.(Tagger); ok {
		return tagger.JawsTags(rq, inTags)
	}
	return append(inTags, t)
}

func (t Template) JawsRender(e *Element, w io.Writer) {
	writeUiDebug(e, w)
	maybePanic(t.Execute(w, With{Element: e, Dot: t.Dot}))
}

func (t Template) JawsUpdate(u Updater) {
	u.Replace(t.ToHTML(u.Element))
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
	return t.ToHTML(e)
}

func (t Template) JawsSet(e *Element, val interface{}) bool {
	panic("Template.JawsSet(): read-only")
}
