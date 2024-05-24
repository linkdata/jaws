package jaws

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"

	"github.com/linkdata/jaws/what"
)

type Template struct {
	*Jaws
	Template string
	Dot      any
}

var _ UI = Template{}           // statically ensure interface is defined
var _ EventHandler = Template{} // statically ensure interface is defined
var _ http.Handler = Template{} // statically ensure interface is defined

func (t Template) String() string {
	return fmt.Sprintf("{%q, %s}", t.Template, TagString(t.Dot))
}

func (t Template) JawsRender(e *Element, wr io.Writer, params []any) error {
	if expandedtags, err := TagExpand(e.Request, t.Dot); err != ErrIllegalTagType {
		e.Request.tagExpanded(e, expandedtags)
	}
	tags, handlers, attrs := ParseParams(params)
	e.Tag(tags...)
	e.handlers = append(e.handlers, handlers...)
	attrstr := template.HTMLAttr(strings.Join(attrs, " ")) // #nosec G203
	return t.Jaws.Lookup(t.Template).Execute(wr, With{
		Element:       e,
		RequestWriter: e.Request.Writer(wr),
		Dot:           t.Dot,
		Attrs:         attrstr,
	})
}

func (t Template) JawsUpdate(e *Element) {
	if dot, ok := t.Dot.(Updater); ok {
		dot.JawsUpdate(e)
	}
}

func (t Template) JawsEvent(e *Element, wht what.What, val string) error {
	return callEventHandler(t.Dot, e, wht, val)
}

// ServeHTTP implements http.Handler.
//
//	http.DefaultServeMux.Handle("/user", myJaws.NewTemplate("user.html", userData))
func (t Template) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	_ = t.Log(t.NewRequest(r).NewElement(t).JawsRender(wr, nil))
}

func (jw *Jaws) NewTemplate(name string, dot any) Template {
	return Template{Jaws: jw, Template: name, Dot: dot}
}
