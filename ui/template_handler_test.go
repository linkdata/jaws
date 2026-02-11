package ui

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/what"
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
	events  int
}

func (d *templateDot) JawsUpdate(*core.Element) {
	d.updated++
}

func (d *templateDot) JawsEvent(*core.Element, what.What, string) error {
	d.events++
	return nil
}

type templateAuth struct{}

func (templateAuth) Data() map[string]any { return map[string]any{"k": "v"} }
func (templateAuth) Email() string        { return "test@example.com" }
func (templateAuth) IsAdmin() bool        { return true }

func TestTemplate_RenderUpdateEventAndHelpers(t *testing.T) {
	jw, rq := newRequest(t)
	log := &templateLogger{}
	jw.Logger = log
	jw.MakeAuth = func(*core.Request) core.Auth { return templateAuth{} }

	jw.AddTemplateLookuper(template.Must(template.New("uitempl").Parse(
		`{{with $.Dot}}<div id="{{$.Jid}}" {{$.Attrs}} data-auth="{{$.Auth.Email}}">{{.}}</div>{{end}}`,
	)))
	jw.AddTemplateLookuper(template.Must(template.New("warn").Parse(`plain`)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}

	if err := rw.Template("uitempl", core.Tag("dot"), "hidden"); err != nil {
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
	if err := tpl.JawsEvent(elem, what.Input, "x"); err != nil {
		t.Fatal(err)
	}
	if td.events != 1 {
		t.Fatalf("expected event call count 1, got %d", td.events)
	}

	if err := rw.Template("warn", core.Tag("x")); err != nil {
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
}

func TestHandler_NewHandlerServeHTTP(t *testing.T) {
	jw, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.AddTemplateLookuper(template.Must(template.New("handler").Parse(`{{with $.Dot}}<div id="{{$.Jid}}">{{.}}</div>{{end}}`)))

	h := NewHandler(jw, "handler", core.Tag("ok"))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	if got := rr.Body.String(); !strings.Contains(got, `<div id="Jid.`) || !strings.Contains(got, `>ok</div>`) {
		t.Fatalf("unexpected handler output: %q", got)
	}
}
