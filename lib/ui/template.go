package ui

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"strings"
	"text/template/parse"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

// Template references a Go [html/template] template to be rendered through JaWS.
//
// The Name field identifies the template to execute and Dot contains the data
// that will be exposed to the template through the [With] structure constructed
// during rendering. Additional tag bindings and event handlers can be supplied
// at render time through the [RequestWriter.Template] helper.
type Template struct {
	Name string // Template name to be looked up using Jaws.LookupTemplate.
	Dot  any    // Dot value to place in With.
}

var _ jaws.UI = Template{}                 // statically ensure interface is defined
var _ jaws.ClickHandler = Template{}       // statically ensure interface is defined
var _ jaws.ContextMenuHandler = Template{} // statically ensure interface is defined
var _ jaws.InputHandler = Template{}       // statically ensure interface is defined

// String returns a debug representation of t.
func (tmpl Template) String() string {
	return fmt.Sprintf("{%q, %s}", tmpl.Name, tag.TagString(tmpl.Dot))
}

func findJidOrJsOrHTMLNode(node parse.Node) (found bool) {
	isJidOrJs := func(s string) bool {
		return (s == "Jid") || (s == "JsFunc") || (s == "JsVar")
	}
	switch node := node.(type) {
	case *parse.TextNode:
		if node != nil {
			found = found || bytes.Contains(node.Text, []byte("</html>"))
		}
	case *parse.ListNode:
		if node != nil {
			for _, n := range node.Nodes {
				found = found || findJidOrJsOrHTMLNode(n)
			}
		}
	case *parse.ActionNode:
		if node != nil {
			found = findJidOrJsOrHTMLNode(node.Pipe)
		}
	case *parse.WithNode:
		if node != nil {
			found = findJidOrJsOrHTMLNode(&node.BranchNode)
		}
	case *parse.BranchNode:
		if node != nil {
			found = findJidOrJsOrHTMLNode(node.Pipe)
			found = found || findJidOrJsOrHTMLNode(node.List)
			found = found || findJidOrJsOrHTMLNode(node.ElseList)
		}
	case *parse.IfNode:
		if node != nil {
			found = findJidOrJsOrHTMLNode(node.Pipe)
			found = found || findJidOrJsOrHTMLNode(node.List)
			found = found || findJidOrJsOrHTMLNode(node.ElseList)
		}
	case *parse.RangeNode:
		if node != nil {
			found = findJidOrJsOrHTMLNode(node.Pipe)
			found = found || findJidOrJsOrHTMLNode(node.List)
			found = found || findJidOrJsOrHTMLNode(node.ElseList)
		}
	case *parse.TemplateNode:
		if node != nil {
			found = findJidOrJsOrHTMLNode(node.Pipe)
		}
	case *parse.PipeNode:
		if node != nil {
			for _, n := range node.Cmds {
				found = found || findJidOrJsOrHTMLNode(n)
			}
		}
	case *parse.CommandNode:
		if node != nil {
			for _, n := range node.Args {
				found = found || findJidOrJsOrHTMLNode(n)
			}
		}
	case *parse.VariableNode:
		if node != nil {
			for _, s := range node.Ident {
				found = found || isJidOrJs(s)
			}
		}
	case *parse.FieldNode:
		if node != nil {
			for _, s := range node.Ident {
				found = found || isJidOrJs(s)
			}
		}
	case *parse.IdentifierNode:
		if node != nil {
			found = found || isJidOrJs(node.Ident)
		}
	case *parse.ChainNode:
		if node != nil {
			found = findJidOrJsOrHTMLNode(node.Node)
			for _, s := range node.Field {
				found = found || isJidOrJs(s)
			}
		}
	}
	return
}

// JawsRender renders t through the request's configured template lookupers.
func (tmpl Template) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	var expandedTags []any
	if expandedTags, err = tag.TagExpand(elem.Request, tmpl.Dot); err == nil {
		elem.Request.TagExpanded(elem, expandedTags)
		tags, handlers, attrs := jaws.ParseParams(params)
		elem.Tag(tags...)
		elem.AddHandlers(handlers...)
		attrstr := template.HTMLAttr(strings.Join(attrs, " ")) // #nosec G203
		var auth jaws.Auth
		auth = jaws.DefaultAuth{}
		if f := elem.Request.Jaws.MakeAuth; f != nil {
			auth = f(elem.Request)
		}
		err = errMissingTemplate(tmpl.Name)
		if lookedUp := elem.Request.Jaws.LookupTemplate(tmpl.Name); lookedUp != nil {
			err = lookedUp.Execute(w, With{
				Element:       elem,
				RequestWriter: RequestWriter{Request: elem.Request, Writer: w},
				Dot:           tmpl.Dot,
				Attrs:         attrstr,
				Auth:          auth,
			})
			if deadlock.Debug && elem.Jaws.Logger != nil {
				if !findJidOrJsOrHTMLNode(lookedUp.Tree.Root) {
					elem.Jaws.Logger.Warn("jaws: template has no Jid reference", "template", tmpl.Name)
				}
			}
		}
	}
	return
}

// JawsUpdate delegates updates to t.Dot when it implements [jaws.Updater].
func (tmpl Template) JawsUpdate(elem *jaws.Element) {
	if dot, ok := tmpl.Dot.(jaws.Updater); ok {
		dot.JawsUpdate(elem)
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
// [template.Template] using [jaws.Jaws.LookupTemplate].
func (rw RequestWriter) Template(name string, dot any, params ...any) error {
	return rw.UI(NewTemplate(name, dot), params...)
}
