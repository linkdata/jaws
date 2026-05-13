package ui

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type templateDot struct {
	updated int
	inputs  int
	clicks  int
	menus   int
}

func (d *templateDot) JawsUpdate(elem *jaws.Element) {
	d.updated++
}

func (d *templateDot) JawsInput(elem *jaws.Element, value string) error {
	d.inputs++
	return nil
}

func (d *templateDot) JawsClick(elem *jaws.Element, click jaws.Click) error {
	d.clicks++
	return nil
}

func (d *templateDot) JawsContextMenu(elem *jaws.Element, click jaws.Click) error {
	d.menus++
	return nil
}

type templateAuth struct{}

func (templateAuth) Data() map[string]any { return map[string]any{"k": "v"} }
func (templateAuth) Email() string        { return "test@example.com" }
func (templateAuth) IsAdmin() bool        { return true }

type templateLogger struct {
	errors []error
}

func (l *templateLogger) Info(string, ...any) {}
func (l *templateLogger) Warn(string, ...any) {}
func (l *templateLogger) Error(_ string, args ...any) {
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "err" {
			if err, ok := args[i+1].(error); ok {
				l.errors = append(l.errors, err)
			}
		}
	}
}

func TestTemplate_RenderUpdateEventAndHelpers(t *testing.T) {
	jw, rq := newCoreRequest(t)
	jw.MakeAuth = func(*jaws.Request) jaws.Auth { return templateAuth{} }

	jw.AddTemplateLookuper(template.Must(template.New("uitempl").Parse(
		`{{with $.Dot}}<span data-auth="{{$.Auth.Email}}">{{.}}</span>{{end}}`,
	)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}

	if err := rw.Template("uitempl", tag.Tag("dot"), "hidden"); err != nil {
		t.Fatal(err)
	}
	got := sb.String()
	if !strings.Contains(got, `<div id="Jid.`) ||
		!strings.Contains(got, `data-jawstemplate hidden`) ||
		!strings.Contains(got, `data-auth="test@example.com"`) ||
		!strings.Contains(got, `>dot</span></div>`) {
		t.Fatalf("unexpected template output: %q", got)
	}

	td := &templateDot{}
	tpl := NewTemplate("uitempl", td)
	if got := tpl.String(); !strings.Contains(got, `{"uitempl", *ui.templateDot(`) {
		t.Fatalf("unexpected template string %q", got)
	}
	elem := rq.NewElement(tpl)
	tpl.JawsUpdate(elem)
	if td.updated != 0 {
		t.Fatalf("expected dot updater not called, got %d", td.updated)
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

	if err := rw.Template("missingtemplate", nil); !errors.Is(err, ErrMissingTemplate) {
		t.Fatalf("expected ErrMissingTemplate, got %v", err)
	}
}

func TestHandler_HandlerServeHTTP(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.AddTemplateLookuper(template.Must(template.New("handler").Parse(
		`<html><body>{{with $.Dot}}<span>{{.}}</span>{{end}}</body></html>`,
	)))

	h := Handler(jw, "handler", tag.Tag("ok"))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	if got := rr.Body.String(); got != `<html><body><span>ok</span></body></html>` {
		t.Fatalf("unexpected handler output: %q", got)
	}
}

func TestTemplate_UpdateLogsMissingTemplate(t *testing.T) {
	jw, rq := newCoreRequest(t)
	logger := new(templateLogger)
	jw.Logger = logger

	tpl := NewTemplate("missingtemplate", tag.Tag("dot"))
	elem := rq.NewElement(tpl)
	tpl.JawsUpdate(elem)

	if len(logger.errors) != 1 {
		t.Fatalf("logged errors = %d, want 1", len(logger.errors))
	}
	if !errors.Is(logger.errors[0], ErrMissingTemplate) {
		t.Fatalf("logged error = %v, want %v", logger.errors[0], ErrMissingTemplate)
	}
}

func TestTemplate_UpdateLogsExecuteError(t *testing.T) {
	jw, rq := newCoreRequest(t)
	logger := new(templateLogger)
	jw.Logger = logger
	jw.AddTemplateLookuper(template.Must(template.New("badupdate").Parse(
		`{{$.Dot.MissingField}}`,
	)))

	tpl := NewTemplate("badupdate", &templateUpdateDot{})
	elem := rq.NewElement(tpl)
	tpl.JawsUpdate(elem)

	if len(logger.errors) != 1 {
		t.Fatalf("logged errors = %d, want 1", len(logger.errors))
	}
	if !strings.Contains(logger.errors[0].Error(), "MissingField") {
		t.Fatalf("logged error = %v, want MissingField error", logger.errors[0])
	}
}

func TestPageTemplate_UpdateNoop(t *testing.T) {
	pageTemplate{}.JawsUpdate(nil)
}

func TestTemplate_RenderReturnsTagExpandError(t *testing.T) {
	jw, rq := newCoreRequest(t)
	jw.AddTemplateLookuper(template.Must(template.New("uitempl").Parse(
		`{{with $.Dot}}<span>{{.}}</span>{{end}}`,
	)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}
	if err := rw.Template("uitempl", "plain-string-dot"); err == nil {
		t.Fatal("expected tag expansion error")
	}
}

type templateUpdateDot struct {
	Text string
}

func TestTemplate_UpdateRerendersIntoWrapper(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	jw.AddTemplateLookuper(template.Must(template.New("update").Parse(
		`{{with $.Dot}}<span>{{.Text}}</span>{{end}}`,
	)))

	go jw.Serve()
	tr := jaws.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	defer tr.Close()
	<-tr.ReadyCh

	dot := &templateUpdateDot{Text: "before"}
	tpl := NewTemplate("update", dot)
	elem := tr.NewElement(tpl)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}
	if got := sb.String(); !strings.Contains(got, `<span>before</span>`) {
		t.Fatalf("unexpected initial render: %q", got)
	}

	dot.Text = "after"
	tr.BcastCh <- wire.Message{Dest: dot, What: what.Update}

	select {
	case msg := <-tr.OutCh:
		if msg.What != what.Inner {
			t.Fatalf("queued update = %v, want %v", msg.What, what.Inner)
		}
		if msg.Jid != elem.Jid() {
			t.Fatalf("queued jid = %v, want %v", msg.Jid, elem.Jid())
		}
		if msg.Data != `<span>after</span>` {
			t.Fatalf("queued inner HTML = %q, want %q", msg.Data, `<span>after</span>`)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for template update")
	}
}
