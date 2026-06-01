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
// The OuterHTMLTag field identifies the generated wrapper element used for
// partial templates. If OuterHTMLTag is empty, the template is rendered without
// a generated wrapper. Name identifies the template to execute and Dot contains
// the data exposed to the template through the [With] structure constructed
// during rendering. Wrapped templates receive the JaWS ID and any HTML
// attributes supplied at render time through the [RequestWriter.Template]
// helper. The referenced template must be a partial template, not a full HTML
// document.
type Template struct {
	OuterHTMLTag string // Optional wrapper tag for partial templates, for example "div" or "tr"; empty renders unwrapped.
	Name         string // Template name to be looked up using Jaws.LookupTemplate.
	Dot          any    // Dot value to place in With.
}

var _ jaws.UI = Template{}                 // statically ensure interface is defined
var _ jaws.ClickHandler = Template{}       // statically ensure interface is defined
var _ jaws.ContextMenuHandler = Template{} // statically ensure interface is defined
var _ jaws.InputHandler = Template{}       // statically ensure interface is defined

// String returns a debug representation of t.
func (tmpl Template) String() string {
	return fmt.Sprintf("{%q, %q, %s}", tmpl.OuterHTMLTag, tmpl.Name, tag.TagString(tmpl.Dot))
}

func (tmpl Template) lookup(elem *jaws.Element) (lookedUp *template.Template, err error) {
	err = errMissingTemplate(tmpl.Name)
	if lookedUp = elem.Request.Jaws.LookupTemplate(tmpl.Name); lookedUp != nil {
		err = nil
	}
	return
}

func (tmpl Template) auth(elem *jaws.Element) (auth jaws.Auth) {
	if f := elem.Request.Jaws.MakeAuth; f != nil {
		auth = f(elem.Request)
	} else {
		auth = &jaws.DefaultAuth{Logger: elem.Request.Jaws.Logger}
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

func writeTemplateWrapperStart(elem *jaws.Element, w io.Writer, outerHTMLTag string, attrs []string) (err error) {
	b := elem.Jid().AppendStartTagAttr(nil, outerHTMLTag)
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

func (tmpl Template) render(elem *jaws.Element, w io.Writer, params []any) (err error) {
	doWrap := tmpl.OuterHTMLTag != ""
	var expandedTags []any
	if expandedTags, err = tag.TagExpand(elem.Request, tmpl.Dot); err == nil {
		elem.Request.TagExpanded(elem, expandedTags)
		tags, handlers, attrs := jaws.ParseParams(params)
		elem.Tag(tags...)
		elem.AddHandlers(handlers...)
		var lookedUp *template.Template
		if lookedUp, err = tmpl.lookup(elem); err == nil {
			if doWrap {
				err = writeTemplateWrapperStart(elem, w, tmpl.OuterHTMLTag, attrs)
			}
			if err == nil {
				if err = tmpl.execute(elem, w, lookedUp); err == nil {
					if doWrap {
						_, err = io.WriteString(w, "</"+tmpl.OuterHTMLTag+">")
					}
				}
			}
		}
	}
	return
}

// JawsRender renders t through the request's configured template lookupers.
func (tmpl Template) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	err = tmpl.render(elem, w, params)
	return
}

// JawsUpdate re-renders t into the template wrapper.
//
// Unwrapped templates have no generated DOM element to update, so updates are
// ignored. Nested JaWS UI rendered by the template can still update through its
// own elements.
//
// Lookup or execution errors are reported through [jaws.Request.MustLog],
// which may panic when no [jaws.Jaws.Logger] is configured.
func (tmpl Template) JawsUpdate(elem *jaws.Element) {
	if tmpl.OuterHTMLTag != "" {
		lookedUp, err := tmpl.lookup(elem)
		if err == nil {
			var sb strings.Builder
			if err = tmpl.execute(elem, &sb, lookedUp); err == nil {
				elem.SetInner(template.HTML(sb.String())) // #nosec G203
			}
		}
		elem.Request.MustLog(err)
	}
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

// NewTemplate returns a Template UI value for rendering the named partial
// template with dot available as [With.Dot].
//
// The template name is resolved when the value is rendered or updated, using
// the request's [jaws.Jaws.LookupTemplate]. If outerHTMLTag is non-empty,
// rendering wraps the template output in that generated element; the wrapper
// owns the JaWS ID and render-time HTML attributes. If outerHTMLTag is empty,
// rendering emits only the template output and [Template.JawsUpdate] has no
// wrapper to update.
//
// Dot participates in tag expansion for dirty targeting and receives delegated
// click, context-menu, and input events when it implements the corresponding
// JaWS handler interfaces.
func NewTemplate(outerHTMLTag, name string, dot any) Template {
	return Template{OuterHTMLTag: outerHTMLTag, Name: name, Dot: dot}
}

// Template renders the given template using [With] as data.
//
// The Dot field in [With] is set to dot, and name is resolved to a
// [template.Template] using [jaws.Jaws.LookupTemplate]. Template output is
// wrapped in a generated outerHTMLTag element that owns the JaWS ID and any
// HTML attrs passed in params. If outerHTMLTag is empty, no wrapper is emitted
// and HTML attr params have no generated element to apply to. The template must
// be a partial, not a full HTML document.
func (rw RequestWriter) Template(outerHTMLTag, name string, dot any, params ...any) error {
	return rw.NewUI(NewTemplate(outerHTMLTag, name, dot), params...)
}
