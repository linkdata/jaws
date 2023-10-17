package jaws

import (
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/linkdata/jaws/what"
)

type Template struct {
	Dot interface{}
	*template.Template
}

var _ UI = Template{}           // statically ensure interface is defined
var _ EventHandler = Template{} // statically ensure interface is defined

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

func (t Template) JawsRender(e *Element, w io.Writer, params []interface{}) {
	if expandedtags, err := TagExpand(e.Request, t.Dot); err != ErrIllegalTagType {
		e.Request.tagExpanded(e, expandedtags)
	}
	var sb strings.Builder
	for _, s := range parseParams(e, params) {
		sb.WriteByte(' ')
		sb.WriteString(s)
	}
	maybePanic(t.Execute(w, With{Element: e, Dot: t.Dot, Attrs: template.HTMLAttr(sb.String())})) // #nosec G203
}

func (t Template) JawsUpdate(e *Element) {
	// does nothing
}

func (t Template) JawsEvent(e *Element, wht what.What, val string) error {
	return callEventHandler(t.Dot, e, wht, val)
}
