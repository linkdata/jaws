package ui

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"strings"
	"text/template/parse"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/what"
)

// Template references a Go html/template to be rendered through JaWS.
//
// The Name field identifies the template to execute and Dot contains the data
// that will be exposed to the template through the With structure constructed
// during rendering. Additional tag bindings and event handlers can be supplied
// at render time through the RequestWriter.Template helper.
type Template struct {
	Name string // Template name to be looked up using Jaws.LookupTemplate()
	Dot  any    // Dot value to place in With structure
}

var _ core.UI = Template{}           // statically ensure interface is defined
var _ core.EventHandler = Template{} // statically ensure interface is defined

func (t Template) String() string {
	return fmt.Sprintf("{%q, %s}", t.Name, core.TagString(t.Dot))
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

func (t Template) JawsRender(e *core.Element, wr io.Writer, params []any) (err error) {
	var expandedtags []any
	if expandedtags, err = core.TagExpand(e.Request, t.Dot); err == nil {
		e.Request.TagExpanded(e, expandedtags)
		tags, handlers, attrs := core.ParseParams(params)
		e.Tag(tags...)
		e.AddHandlers(handlers...)
		attrstr := template.HTMLAttr(strings.Join(attrs, " ")) // #nosec G203
		var auth core.Auth
		auth = core.DefaultAuth{}
		if f := e.Request.Jaws.MakeAuth; f != nil {
			auth = f(e.Request)
		}
		err = errMissingTemplate(t.Name)
		if tmpl := e.Request.Jaws.LookupTemplate(t.Name); tmpl != nil {
			err = tmpl.Execute(wr, With{
				Element:       e,
				RequestWriter: RequestWriter{Request: e.Request, Writer: wr},
				Dot:           t.Dot,
				Attrs:         attrstr,
				Auth:          auth,
			})
			if deadlock.Debug && e.Jaws.Logger != nil {
				if !findJidOrJsOrHTMLNode(tmpl.Tree.Root) {
					e.Jaws.Logger.Warn("jaws: template has no Jid reference", "template", t.Name)
				}
			}
		}
	}
	return
}

func (t Template) JawsUpdate(e *core.Element) {
	if dot, ok := t.Dot.(core.Updater); ok {
		dot.JawsUpdate(e)
	}
}

func (t Template) JawsEvent(e *core.Element, wht what.What, val string) error {
	return core.CallEventHandlers(t.Dot, e, wht, val)
}

// NewTemplate constructs a Template with the provided name and data value.
//
// It is a small helper that makes it convenient to use Template values with
// other JaWS helpers without having to fill the struct fields manually.
func NewTemplate(name string, dot any) Template {
	return Template{Name: name, Dot: dot}
}

// Template renders the given template using ui.With as data.
//
// The Dot field in ui.With is set to dot, and name is resolved to a
// *template.Template using Jaws.LookupTemplate().
func (rqw RequestWriter) Template(name string, dot any, params ...any) error {
	return rqw.UI(NewTemplate(name, dot), params...)
}
