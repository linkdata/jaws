package jaws

import (
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/linkdata/jaws/what"
)

type Template struct {
	Dot      any
	Template any
}

var _ UI = Template{}           // statically ensure interface is defined
var _ EventHandler = Template{} // statically ensure interface is defined

// MustTemplate resolves 'v' to a *template.Template or panics.
func (rq *Request) MustTemplate(v any) (tp *template.Template) {
	switch v := v.(type) {
	case *template.Template:
		tp = v
	case string:
		tp = rq.Jaws.Lookup(v)
	}
	if tp == nil {
		panic(fmt.Errorf("template not found: %T(%v)", v, v))
	}
	return
}

func (rq *Request) MakeTemplate(templ, dot any) Template {
	return Template{Template: templ, Dot: dot}
}

func (t Template) String() string {
	return fmt.Sprintf("{%q, %s}", t.Template, TagString(t.Dot))
}

func (t Template) JawsRender(e *Element, w io.Writer, params []any) error {
	if expandedtags, err := TagExpand(e.Request, t.Dot); err != ErrIllegalTagType {
		e.Request.tagExpanded(e, expandedtags)
	}
	var sb strings.Builder
	for _, s := range e.ParseParams(params) {
		sb.WriteByte(' ')
		sb.WriteString(string(s))
	}
	attrs := template.HTMLAttr(sb.String()) // #nosec G203
	return e.Request.MustTemplate(t.Template).Execute(w, With{
		Element:       e,
		RequestWriter: e.Request.Writer(w),
		Dot:           t.Dot,
		Attrs:         attrs,
	})
}

func (t Template) JawsUpdate(e *Element) {
	// does nothing
}

func (t Template) JawsEvent(e *Element, wht what.What, val string) error {
	return callEventHandler(t.Dot, e, wht, val)
}
