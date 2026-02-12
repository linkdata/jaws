package core

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"text/template/parse"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

type testRequestWriter struct {
	rq *Request
	io.Writer
}

type testRegisterUI struct{ Updater }

func (testRegisterUI) JawsRender(*Element, io.Writer, []any) error { return nil }
func (ui testRegisterUI) JawsUpdate(e *Element)                    { ui.Updater.JawsUpdate(e) }

func (rw testRequestWriter) UI(ui UI, params ...any) error {
	return rw.rq.NewElement(ui).JawsRender(rw, params)
}

func (rw testRequestWriter) Write(p []byte) (n int, err error) {
	rw.rq.Rendering.Store(true)
	return rw.Writer.Write(p)
}

func (rw testRequestWriter) Request() *Request {
	return rw.rq
}

func (rw testRequestWriter) Initial() *http.Request {
	return rw.rq.Initial()
}

func (rw testRequestWriter) HeadHTML() error {
	return rw.rq.HeadHTML(rw)
}

func (rw testRequestWriter) TailHTML() error {
	return rw.rq.TailHTML(rw)
}

func (rw testRequestWriter) Session() *Session {
	return rw.rq.Session()
}

func (rw testRequestWriter) Get(key string) (val any) {
	return rw.rq.Get(key)
}

func (rw testRequestWriter) Set(key string, val any) {
	rw.rq.Set(key, val)
}

func (rw testRequestWriter) Register(updater Updater, params ...any) Jid {
	elem := rw.rq.NewElement(testRegisterUI{Updater: updater})
	elem.Tag(updater)
	elem.ApplyParams(params)
	updater.JawsUpdate(elem)
	return elem.Jid()
}

func (rq *Request) Writer(w io.Writer) testRequestWriter {
	return testRequestWriter{rq: rq, Writer: w}
}

type testHandler struct {
	*Jaws
	Template testTemplateUI
}

func (h testHandler) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	_ = h.Log(h.NewRequest(r).NewElement(h.Template).JawsRender(wr, nil))
}

func (jw *Jaws) Handler(name string, dot any) http.Handler {
	return testHandler{Jaws: jw, Template: testTemplateUI{Name: name, Dot: dot}}
}

type testWith struct {
	*Element
	testRequestWriter
	Dot   any
	Attrs template.HTMLAttr
	Auth  Auth
}

type testTemplateUI struct {
	Name string
	Dot  any
}

func (t testTemplateUI) String() string {
	return fmt.Sprintf("{%q, %s}", t.Name, TagString(t.Dot))
}

func findJidOrJsOrHTMLNode(node parse.Node) (found bool) {
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
				found = found || (s == "Jid") || (s == "JsFunc") || (s == "JsVar")
			}
		}
	}
	return
}

func (t testTemplateUI) JawsRender(e *Element, wr io.Writer, params []any) (err error) {
	var expandedtags []any
	if expandedtags, err = TagExpand(e.Request, t.Dot); err == nil {
		e.Request.TagExpanded(e, expandedtags)
		tags, handlers, attrs := ParseParams(params)
		e.Tag(tags...)
		e.AddHandlers(handlers...)
		attrstr := template.HTMLAttr(strings.Join(attrs, " ")) // #nosec G203
		var auth Auth = DefaultAuth{}
		if f := e.Request.Jaws.MakeAuth; f != nil {
			auth = f(e.Request)
		}
		err = fmt.Errorf("missing template %q", t.Name)
		if tmpl := e.Request.Jaws.LookupTemplate(t.Name); tmpl != nil {
			err = tmpl.Execute(wr, testWith{
				Element:           e,
				testRequestWriter: testRequestWriter{rq: e.Request, Writer: wr},
				Dot:               t.Dot,
				Attrs:             attrstr,
				Auth:              auth,
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

func (t testTemplateUI) JawsUpdate(e *Element) {
	if dot, ok := t.Dot.(Updater); ok {
		dot.JawsUpdate(e)
	}
}

func (t testTemplateUI) JawsEvent(e *Element, wht what.What, val string) error {
	return CallEventHandlers(t.Dot, e, wht, val)
}

func (rw testRequestWriter) Template(name string, dot any, params ...any) error {
	return rw.UI(testTemplateUI{Name: name, Dot: dot}, params...)
}
