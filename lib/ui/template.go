package ui

import (
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

// Template references a Go [html/template] template to be rendered through JaWS.
//
// The Name field identifies the template to execute and Dot contains the data
// that will be exposed to the template through the [With] structure constructed
// during rendering. Templates are rendered inside a generated wrapper element
// that receives the JaWS ID and any HTML attributes supplied at render time
// through the [RequestWriter.Template] helper. The referenced template must be
// a partial template, not a full HTML document.
type Template struct {
	Name string // Template name to be looked up using Jaws.LookupTemplate.
	Dot  any    // Dot value to place in With.
}

var _ jaws.UI = Template{}                 // statically ensure interface is defined
var _ jaws.ClickHandler = Template{}       // statically ensure interface is defined
var _ jaws.ContextMenuHandler = Template{} // statically ensure interface is defined
var _ jaws.InputHandler = Template{}       // statically ensure interface is defined

type templateRenderMode uint8

const (
	templateRenderWrapped templateRenderMode = iota
	templateRenderDirect
)

// String returns a debug representation of t.
func (tmpl Template) String() string {
	return fmt.Sprintf("{%q, %s}", tmpl.Name, tag.TagString(tmpl.Dot))
}

func (tmpl Template) lookup(elem *jaws.Element) (lookedUp *template.Template, err error) {
	err = errMissingTemplate(tmpl.Name)
	if lookedUp = elem.Request.Jaws.LookupTemplate(tmpl.Name); lookedUp != nil {
		err = nil
	}
	return
}

func (tmpl Template) auth(elem *jaws.Element) (auth jaws.Auth) {
	auth = jaws.DefaultAuth{}
	if f := elem.Request.Jaws.MakeAuth; f != nil {
		auth = f(elem.Request)
	}
	return
}

func (tmpl Template) execute(elem *jaws.Element, w io.Writer, lookedUp *template.Template) (err error) {
	err = lookedUp.Execute(w, With{
		Element:       elem,
		RequestWriter: RequestWriter{Request: elem.Request, Writer: w},
		Dot:           tmpl.Dot,
		Auth:          tmpl.auth(elem),
	})
	return
}

func writeTemplateWrapperStart(elem *jaws.Element, w io.Writer, attrs []string) (err error) {
	b := elem.Jid().AppendStartTagAttr(nil, "div")
	b = append(b, " data-jawstemplate"...)
	for _, attr := range attrs {
		if attr != "" {
			b = append(b, ' ')
			b = append(b, attr...)
		}
	}
	b = append(b, '>')
	_, err = w.Write(b)
	return
}

func (tmpl Template) render(elem *jaws.Element, w io.Writer, params []any, wrapped bool) (err error) {
	var expandedTags []any
	if expandedTags, err = tag.TagExpand(elem.Request, tmpl.Dot); err == nil {
		elem.Request.TagExpanded(elem, expandedTags)
		tags, handlers, attrs := jaws.ParseParams(params)
		elem.Tag(tags...)
		elem.AddHandlers(handlers...)
		var lookedUp *template.Template
		if lookedUp, err = tmpl.lookup(elem); err == nil {
			if wrapped {
				err = writeTemplateWrapperStart(elem, w, attrs)
			}
			if err == nil {
				if err = tmpl.execute(elem, w, lookedUp); err == nil {
					if wrapped {
						_, err = io.WriteString(w, "</div>")
					}
				}
			}
		}
	}
	return
}

// JawsRender renders t through the request's configured template lookupers.
func (tmpl Template) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	err = tmpl.render(elem, w, params, true)
	return
}

// JawsUpdate re-renders t into the template wrapper.
func (tmpl Template) JawsUpdate(elem *jaws.Element) {
	lookedUp, err := tmpl.lookup(elem)
	if err == nil {
		var sb strings.Builder
		if err = tmpl.execute(elem, &sb, lookedUp); err == nil {
			elem.SetInner(template.HTML(sb.String())) // #nosec G203
		}
	}
	elem.Request.MustLog(err)
}

// JawsClick delegates click events to t.Dot when it implements [jaws.ClickHandler].
func (tmpl Template) JawsClick(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	if h, ok := tmpl.Dot.(jaws.ClickHandler); ok {
		err = h.JawsClick(elem, click)
	}
	return
}

// JawsContextMenu delegates context-menu events to t.Dot when it implements
// [jaws.ContextMenuHandler].
func (tmpl Template) JawsContextMenu(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	if h, ok := tmpl.Dot.(jaws.ContextMenuHandler); ok {
		err = h.JawsContextMenu(elem, click)
	}
	return
}

// JawsInput delegates input events to t.Dot when it implements [jaws.InputHandler].
func (tmpl Template) JawsInput(elem *jaws.Element, value string) (err error) {
	err = jaws.ErrEventUnhandled
	if h, ok := tmpl.Dot.(jaws.InputHandler); ok {
		err = h.JawsInput(elem, value)
	}
	return
}

// NewTemplate constructs a Template with the provided name and data value.
//
// It is a small helper that makes it convenient to use Template values with
// other JaWS helpers without having to fill the struct fields manually.
func NewTemplate(name string, dot any) Template {
	return Template{Name: name, Dot: dot}
}

// Template renders the given template using [With] as data.
//
// The Dot field in [With] is set to dot, and name is resolved to a
// [template.Template] using [jaws.Jaws.LookupTemplate]. Template output is
// wrapped in a generated div that owns the JaWS ID and any HTML attrs passed in
// params. The template must be a partial, not a full HTML document.
func (rw RequestWriter) Template(name string, dot any, params ...any) error {
	return rw.UI(NewTemplate(name, dot), params...)
}
