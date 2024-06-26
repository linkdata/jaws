package jaws

import (
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/linkdata/jaws/what"
)

type Template struct {
	Name string // Template name to be looked up using jaws.LookupTemplate()
	Dot  any    // Dot value to place in With structure
}

var _ UI = Template{}           // statically ensure interface is defined
var _ EventHandler = Template{} // statically ensure interface is defined

func (t Template) String() string {
	return fmt.Sprintf("{%q, %s}", t.Name, TagString(t.Dot))
}

func (t Template) JawsRender(e *Element, wr io.Writer, params []any) error {
	if expandedtags, err := TagExpand(e.Request, t.Dot); err != ErrIllegalTagType {
		e.Request.tagExpanded(e, expandedtags)
	}
	tags, handlers, attrs := ParseParams(params)
	e.Tag(tags...)
	e.handlers = append(e.handlers, handlers...)
	attrstr := template.HTMLAttr(strings.Join(attrs, " ")) // #nosec G203
	return e.Request.Jaws.LookupTemplate(t.Name).Execute(wr, With{
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
	return callEventHandlers(t.Dot, e, wht, val)
}

// NewTemplate simply returns a Template{} with the members set.
//
// Provided as convenience so as to not have to name the structure members.
func NewTemplate(name string, dot any) Template {
	return Template{Name: name, Dot: dot}
}

// Template renders the given template using jaws.With{Dot: dot} as data.
//
// The name argument is a string to be resolved to a *template.Template
// using Jaws.LookupTemplate().
func (rq RequestWriter) Template(name string, dot any, params ...any) error {
	return rq.UI(NewTemplate(name, dot), params...)
}
