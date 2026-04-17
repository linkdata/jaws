package ui

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"text/template/parse"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
)

type templateLogger struct {
	warns  int
	errors int
}

func (l *templateLogger) Info(string, ...any)  {}
func (l *templateLogger) Warn(string, ...any)  { l.warns++ }
func (l *templateLogger) Error(string, ...any) { l.errors++ }

type templateDot struct {
	updated int
	inputs  int
	clicks  int
	menus   int
}

func (d *templateDot) JawsUpdate(*jaws.Element) {
	d.updated++
}

func (d *templateDot) JawsInput(*jaws.Element, string) error {
	d.inputs++
	return nil
}

func (d *templateDot) JawsClick(*jaws.Element, jaws.Click) error {
	d.clicks++
	return nil
}

func (d *templateDot) JawsContextMenu(*jaws.Element, jaws.Click) error {
	d.menus++
	return nil
}

type templateAuth struct{}

func (templateAuth) Data() map[string]any { return map[string]any{"k": "v"} }
func (templateAuth) Email() string        { return "test@example.com" }
func (templateAuth) IsAdmin() bool        { return true }

func TestTemplate_RenderUpdateEventAndHelpers(t *testing.T) {
	jw, rq := newCoreRequest(t)
	log := &templateLogger{}
	jw.Logger = log
	jw.MakeAuth = func(*jaws.Request) jaws.Auth { return templateAuth{} }

	jw.AddTemplateLookuper(template.Must(template.New("uitempl").Parse(
		`{{with $.Dot}}<div id="{{$.Jid}}" {{$.Attrs}} data-auth="{{$.Auth.Email}}">{{.}}</div>{{end}}`,
	)))
	jw.AddTemplateLookuper(template.Must(template.New("warn").Parse(`plain`)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}

	if err := rw.Template("uitempl", tag.Tag("dot"), "hidden"); err != nil {
		t.Fatal(err)
	}
	got := sb.String()
	if !strings.Contains(got, `<div id="Jid.`) ||
		!strings.Contains(got, `hidden`) ||
		!strings.Contains(got, `data-auth="test@example.com"`) ||
		!strings.Contains(got, `>dot</div>`) {
		t.Fatalf("unexpected template output: %q", got)
	}

	td := &templateDot{}
	tpl := NewTemplate("uitempl", td)
	if got := tpl.String(); !strings.Contains(got, `{"uitempl", *ui.templateDot(`) {
		t.Fatalf("unexpected template string %q", got)
	}
	elem := rq.NewElement(tpl)
	tpl.JawsUpdate(elem)
	if td.updated != 1 {
		t.Fatalf("expected updater called once, got %d", td.updated)
	}
	if err := tpl.JawsInput(elem, "x"); err != nil {
		t.Fatal(err)
	}
	if err := tpl.JawsClick(elem, jaws.Click{Name: "btn"}); err != nil {
		t.Fatal(err)
	}
	if err := tpl.JawsContextMenu(elem, jaws.Click{Name: "ctx"}); err != nil {
		t.Fatal(err)
	}
	if err := jaws.CallEventHandlers(tpl, elem, what.Set, "path=1"); err != nil {
		t.Fatal(err)
	}
	if td.inputs != 2 {
		t.Fatalf("expected input call count 2, got %d", td.inputs)
	}
	if td.clicks != 1 {
		t.Fatalf("expected click call count 1, got %d", td.clicks)
	}
	if td.menus != 1 {
		t.Fatalf("expected context-menu call count 1, got %d", td.menus)
	}

	if err := rw.Template("warn", tag.Tag("x")); err != nil {
		t.Fatal(err)
	}
	if deadlock.Debug && log.warns == 0 {
		t.Fatal("expected warning for template without Jid/Js references")
	}

	if err := rw.Template("missingtemplate", nil); !errors.Is(err, ErrMissingTemplate) {
		t.Fatalf("expected ErrMissingTemplate, got %v", err)
	}
}

func TestTemplate_findJidOrJsOrHTMLNode(t *testing.T) {
	if findJidOrJsOrHTMLNode(nil) {
		t.Fatal("nil node should not match")
	}

	plain := template.Must(template.New("plain").Parse(`plain text`))
	if findJidOrJsOrHTMLNode(plain.Tree.Root) {
		t.Fatal("plain text should not match")
	}

	htmlNode := template.Must(template.New("html").Parse(`</html>`))
	if !findJidOrJsOrHTMLNode(htmlNode.Tree.Root) {
		t.Fatal("expected html close marker match")
	}

	complex := template.Must(template.New("complex").Parse(
		`{{if .}}{{with .}}{{$.Jid}}{{$.JsVar}}{{$.JsFunc}}{{end}}{{end}}`,
	))
	if !findJidOrJsOrHTMLNode(complex.Tree.Root) {
		t.Fatal("expected Jid/Js node match")
	}

	rangeNode := template.Must(template.New("range").Parse(`{{range .}}{{$.Jid}}{{end}}`))
	if !findJidOrJsOrHTMLNode(rangeNode.Tree.Root) {
		t.Fatal("expected range node Jid match")
	}

	templateNode := &parse.TemplateNode{
		Pipe: &parse.PipeNode{
			Cmds: []*parse.CommandNode{{
				Args: []parse.Node{
					&parse.VariableNode{Ident: []string{"$", "Jid"}},
				},
			}},
		},
	}
	if !findJidOrJsOrHTMLNode(templateNode) {
		t.Fatal("expected template node Jid match")
	}

	if !findJidOrJsOrHTMLNode(&parse.FieldNode{Ident: []string{"Jid"}}) {
		t.Fatal("expected field node Jid match")
	}

	if !findJidOrJsOrHTMLNode(&parse.IdentifierNode{Ident: "JsVar"}) {
		t.Fatal("expected identifier node JsVar match")
	}

	if !findJidOrJsOrHTMLNode(&parse.ChainNode{
		Node:  &parse.VariableNode{Ident: []string{"$"}},
		Field: []string{"JsFunc"},
	}) {
		t.Fatal("expected chain node JsFunc match")
	}
}

func TestHandler_HandlerServeHTTP(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.AddTemplateLookuper(template.Must(template.New("handler").Parse(`{{with $.Dot}}<div id="{{$.Jid}}">{{.}}</div>{{end}}`)))

	h := Handler(jw, "handler", tag.Tag("ok"))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	if got := rr.Body.String(); !strings.Contains(got, `<div id="Jid.`) || !strings.Contains(got, `>ok</div>`) {
		t.Fatalf("unexpected handler output: %q", got)
	}
}

func TestTemplate_RenderReturnsTagExpandError(t *testing.T) {
	jw, rq := newCoreRequest(t)
	jw.AddTemplateLookuper(template.Must(template.New("uitempl").Parse(
		`{{with $.Dot}}<div id="{{$.Jid}}" {{$.Attrs}}>{{.}}</div>{{end}}`,
	)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}
	if err := rw.Template("uitempl", "plain-string-dot"); err == nil {
		t.Fatal("expected tag expansion error")
	}
}
