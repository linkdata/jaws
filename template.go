package jaws

import (
	"fmt"
	"html/template"
	"io"
	"strings"
	"text/template/parse"

	"github.com/linkdata/deadlock"
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

func findJidOrJs(node parse.Node) (found bool) {
	switch node := node.(type) {
	case *parse.ListNode:
		if node != nil {
			for _, n := range node.Nodes {
				found = found || findJidOrJs(n)
			}
		}
	case *parse.ActionNode:
		if node != nil {
			found = findJidOrJs(node.Pipe)
		}
	case *parse.WithNode:
		if node != nil {
			found = findJidOrJs(&node.BranchNode)
		}
	case *parse.BranchNode:
		if node != nil {
			found = findJidOrJs(node.Pipe)
			found = found || findJidOrJs(node.List)
			found = found || findJidOrJs(node.ElseList)
		}
	case *parse.IfNode:
		if node != nil {
			found = findJidOrJs(node.Pipe)
			found = found || findJidOrJs(node.List)
			found = found || findJidOrJs(node.ElseList)
		}
	case *parse.PipeNode:
		if node != nil {
			for _, n := range node.Cmds {
				found = found || findJidOrJs(n)
			}
		}
	case *parse.CommandNode:
		if node != nil {
			for _, n := range node.Args {
				found = found || findJidOrJs(n)
			}
		}
	case *parse.VariableNode:
		if node != nil {
			for _, s := range node.Ident {
				found = found || (s == "Jid") || (s == "JsFunc") || (s == "JsVar")
			}
		}
	}
	return
}

func (t Template) JawsRender(e *Element, wr io.Writer, params []any) (err error) {
	var expandedtags []any
	if expandedtags, err = TagExpand(e.Request, t.Dot); err == nil {
		e.Request.tagExpanded(e, expandedtags)
		tags, handlers, attrs := ParseParams(params)
		e.Tag(tags...)
		e.handlers = append(e.handlers, handlers...)
		attrstr := template.HTMLAttr(strings.Join(attrs, " ")) // #nosec G203
		var auth Auth
		auth = defaultAuth{}
		if f := e.Request.Jaws.MakeAuth; f != nil {
			auth = f(e.Request)
		}
		err = errMissingTemplate(t.Name)
		if tmpl := e.Request.Jaws.LookupTemplate(t.Name); tmpl != nil {
			err = tmpl.Execute(wr, With{
				Element:       e,
				RequestWriter: e.Request.Writer(wr),
				Dot:           t.Dot,
				Attrs:         attrstr,
				Auth:          auth,
			})
			if deadlock.Debug && e.Jaws.Logger != nil {
				if !findJidOrJs(tmpl.Tree.Root) {
					e.Jaws.Logger.Warn("jaws: template has no Jid reference", "template", t.Name)
				}
			}
		}
	}
	return
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
