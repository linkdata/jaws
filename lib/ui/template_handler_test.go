package ui

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
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

type templateInitialAttrDot struct {
	mu    sync.Mutex
	calls []jaws.Jid
}

type templateStaticInitialAttrDot struct {
	attr template.HTMLAttr
}

func (d *templateInitialAttrDot) JawsInitialHTMLAttr(elem *jaws.Element) template.HTMLAttr {
	d.mu.Lock()
	d.calls = append(d.calls, elem.Jid())
	d.mu.Unlock()
	return template.HTMLAttr(`data-element="` + elem.Jid().String() + `"`)
}

func (d *templateInitialAttrDot) attrCalls() (calls []jaws.Jid) {
	d.mu.Lock()
	calls = append(calls, d.calls...)
	d.mu.Unlock()
	return
}

func (d templateStaticInitialAttrDot) JawsInitialHTMLAttr(*jaws.Element) template.HTMLAttr {
	return d.attr
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
	log testErrorLog
}

func (l *templateLogger) Info(string, ...any) {}
func (l *templateLogger) Warn(string, ...any) {}
func (l *templateLogger) Error(_ string, args ...any) {
	l.log.record(args)
}

func (l *templateLogger) sync(t *testing.T, jw *jaws.Jaws) []error {
	return l.log.sync(t, jw)
}

// warnCountLogger counts Warn calls whose message contains substr.
type warnCountLogger struct {
	substr string
	count  int
}

func (l *warnCountLogger) Info(string, ...any) {}
func (l *warnCountLogger) Warn(msg string, _ ...any) {
	if strings.Contains(msg, l.substr) {
		l.count++
	}
}
func (l *warnCountLogger) Error(string, ...any) {}

// TestTemplate_DefaultAuthWarnsOncePerJawsAcrossRenders verifies that with
// MakeAuth unset, rendering a template that consults .Auth.IsAdmin logs the
// fail-open warning only once per Jaws instance across many renders. The reused
// jaws.Jaws.DefaultAuth keeps its sync.Once effective; a fresh DefaultAuth
// allocated per render (the previous behavior) re-warns on every render.
func TestTemplate_DefaultAuthWarnsOncePerJawsAcrossRenders(t *testing.T) {
	jw, rq := newCoreRequest(t)
	logger := &warnCountLogger{substr: "DefaultAuth.IsAdmin returns true"}
	jw.Logger = logger
	// MakeAuth is deliberately left nil so templates receive the DefaultAuth.

	_ = jw.AddTemplateLookuper(template.Must(template.New("authtmpl").Parse(
		`{{if $.Auth.IsAdmin}}<span>admin</span>{{end}}`,
	)))

	for range 3 {
		var sb bytes.Buffer
		rw := RequestWriter{Request: rq, Writer: &sb}
		if err := rw.Template("div", "authtmpl", tag.Tag("dot")); err != nil {
			t.Fatal(err)
		}
	}

	if logger.count != 1 {
		t.Fatalf("fail-open warning logged %d times across 3 renders, want 1", logger.count)
	}
}

func TestTemplate_RenderUpdateEventAndHelpers(t *testing.T) {
	jw, rq := newCoreRequest(t)
	jw.MakeAuth = func(*jaws.Request) jaws.Auth { return templateAuth{} }

	_ = jw.AddTemplateLookuper(template.Must(template.New("uitempl").Parse(
		`{{with $.Dot}}<span data-auth="{{$.Auth.Email}}">{{.}}</span>{{end}}`,
	)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}

	if err := rw.Template("div", "uitempl", tag.Tag("dot"), "hidden"); err != nil {
		t.Fatal(err)
	}
	got := sb.String()
	if !strings.Contains(got, `<div id="Jid.`) ||
		!strings.Contains(got, ` hidden>`) ||
		!strings.Contains(got, `data-auth="test@example.com"`) ||
		!strings.Contains(got, `>dot</span></div>`) {
		t.Fatalf("unexpected template output: %q", got)
	}

	td := &templateDot{}
	tpl := NewTemplate("div", "uitempl", td)
	// Dot is rendered with tag.TagString, whose exact form varies by build, so build
	// the expectation the same way rather than hardcoding it.
	if got, want := tpl.String(), `{"div", "uitempl", `+tag.TagString(td); !strings.Contains(got, want) {
		t.Fatalf("template string %q does not contain %q", got, want)
	}
	// Render before updating: a wrapped Template claims its state slot while rendering and
	// reports ErrElementStateUnclaimed for an Element it never rendered.
	elem := rq.NewElement(tpl)
	if err := elem.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}
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

	if err := rw.Template("div", "missingtemplate", nil); !errors.Is(err, ErrMissingTemplate) {
		t.Fatalf("expected ErrMissingTemplate, got %v", err)
	}
}

func TestNewTemplate_RendersDotInitialHTMLAttrPerElement(t *testing.T) {
	jw, rq := newCoreRequest(t)
	if err := jw.AddTemplateLookuper(template.Must(template.New("attrtmpl").Parse(`content`))); err != nil {
		t.Fatal(err)
	}

	dot := new(templateInitialAttrDot)
	first := NewTemplate("article", "attrtmpl", dot, `class="card"`, "hidden")
	second := NewTemplate("article", "attrtmpl", dot, `class="card" hidden`)
	if first != second {
		t.Fatal("Templates rebuilt from the same definition are not equal")
	}
	definitions := map[Template]struct{}{first: {}}
	if _, ok := definitions[second]; !ok {
		t.Fatal("equal Template is not usable as a map key")
	}

	provider := &testContainer{contents: []jaws.UI{first, second}}
	containerElem, got := renderUI(t, rq, NewContainer("section", provider))
	children := containerElements(t, containerElem)
	if len(children) != 2 {
		t.Fatalf("Template children = %d, want 2", len(children))
	}
	if templateStateOf(children[0]) == templateStateOf(children[1]) {
		t.Fatal("equal Templates share per-Element state")
	}
	for _, child := range children {
		want := `<article id="` + child.Jid().String() + `" class="card" hidden data-element="` + child.Jid().String() + `">content</article>`
		if !strings.Contains(got, want) {
			t.Errorf("rendered HTML %q does not contain %q", got, want)
		}
	}

	calls := dot.attrCalls()
	if len(calls) != len(children) {
		t.Fatalf("JawsInitialHTMLAttr calls = %v, want one per Template Element", calls)
	}
	for i, child := range children {
		if calls[i] != child.Jid() {
			t.Errorf("JawsInitialHTMLAttr call %d used Element %v, want %v", i, calls[i], child.Jid())
		}
	}

	children[0].JawsUpdate()
	if calls = dot.attrCalls(); len(calls) != len(children) {
		t.Fatalf("JawsInitialHTMLAttr calls after update = %v, want initial-render calls only", calls)
	}
}

func TestNewTemplate_JoinsAttrsForValueIdentity(t *testing.T) {
	dot := tag.Tag("dot")
	attrs := []string{`class="card"`, "hidden"}
	fromParts := NewTemplate("article", "attrtmpl", dot, attrs...)
	fromFragment := NewTemplate("article", "attrtmpl", dot, `class="card" hidden`)
	attrs[0] = `class="changed"`
	if fromParts != fromFragment {
		t.Fatal("equivalent rendered attributes produce unequal Templates")
	}
	definitions := map[Template]struct{}{fromParts: {}}
	if _, ok := definitions[fromFragment]; !ok {
		t.Fatal("Template with constructor attributes is not usable as a map key")
	}
	if got := fromParts.String(); !strings.Contains(got, `class=\"card\" hidden`) {
		t.Fatalf("Template string %q omits its constructor attributes", got)
	}
	if changed := NewTemplate("article", "attrtmpl", dot, `class="changed" hidden`); fromParts == changed {
		t.Fatal("different rendered attributes produce equal Templates")
	}
}

func TestTemplate_RenderAttributePrecedence(t *testing.T) {
	jw, rq := newCoreRequest(t)
	if err := jw.AddTemplateLookuper(template.Must(template.New("attrtmpl").Parse(`content`))); err != nil {
		t.Fatal(err)
	}

	dot := templateStaticInitialAttrDot{attr: `class="dot" data-dot="yes"`}
	_, got := renderUI(t, rq,
		NewTemplate("article", "attrtmpl", dot, `class="constructor"`, `data-constructor="yes"`),
		template.HTMLAttr(`class="param" data-param="yes"`),
	)
	want := `class="param" data-param="yes" class="constructor" data-constructor="yes" class="dot" data-dot="yes"`
	if !strings.Contains(got, want) {
		t.Fatalf("rendered HTML does not contain ordered attributes %q: %q", want, got)
	}
}

func TestTemplate_RenderWithTableRowWrapper(t *testing.T) {
	jw, rq := newCoreRequest(t)
	// The native template action includes the structural td fragment without another
	// JaWS wrapper; the managed row Template supplies the one addressable tr.
	_ = jw.AddTemplateLookuper(template.Must(template.New("row").Parse(
		`{{template "cell" .}}{{define "cell"}}<td>{{.Dot}}</td>{{end}}`,
	)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}
	if err := rw.Template("tr", "row", tag.Tag("cell"), `class="selected"`); err != nil {
		t.Fatal(err)
	}
	got := sb.String()
	if !strings.HasPrefix(got, `<tr id="Jid.`) ||
		!strings.Contains(got, ` class="selected"`) ||
		!strings.HasSuffix(got, `<td>cell</td></tr>`) {
		t.Fatalf("unexpected table row template output: %q", got)
	}
}

func TestTemplate_RenderWithDefaultWrapper(t *testing.T) {
	jw, rq := newCoreRequest(t)
	_ = jw.AddTemplateLookuper(template.Must(template.New("bare").Parse(
		`<span>{{.Dot}}</span>`,
	)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}
	dot := tag.Tag("cell")
	if err := rw.Template("", "bare", dot, `class="defaulted"`); err != nil {
		t.Fatal(err)
	}
	elems := rq.GetElements(dot)
	if len(elems) != 1 {
		t.Fatalf("elements tagged with %q = %d, want 1", dot, len(elems))
	}
	want := `<div id="` + elems[0].Jid().String() + `" class="defaulted"><span>cell</span></div>`
	if got := sb.String(); got != want {
		t.Fatalf("default-wrapped template output = %q, want %q", got, want)
	}
}

// TestTemplate_DirectEmptyWrapperRendersUnwrapped preserves the raw path used by
// private construction and the zero value. Public callers using NewTemplate or
// RequestWriter.Template get the default wrapper covered above.
func TestTemplate_DirectEmptyWrapperRendersUnwrapped(t *testing.T) {
	jw, rq := newCoreRequest(t)
	_ = jw.AddTemplateLookuper(template.Must(template.New("bare").Parse(
		`<span>{{.Dot}}</span>`,
	)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}
	tmpl := newTemplate("", "bare", tag.Tag("cell"))
	if err := rw.NewUI(tmpl, `class="ignored"`); err != nil {
		t.Fatal(err)
	}
	if got, want := sb.String(), `<span>cell</span>`; got != want {
		t.Fatalf("direct empty-wrapper output = %q, want %q", got, want)
	}
}

func TestHandler_HandlerServeHTTP(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	_ = jw.AddTemplateLookuper(template.Must(template.New("handler").Parse(
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

func TestHandler_RenderErrorDoesNotLeakElement(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := new(templateLogger)
	jw.Logger = logger

	renderErr := errors.New("render failed")
	var captured *jaws.Request
	_ = jw.AddTemplateLookuper(template.Must(template.New("handlerfail").Funcs(template.FuncMap{
		"capture": func(w With) string {
			captured = w.RequestWriter.Request
			return ""
		},
		"fail": func() (string, error) {
			return "", renderErr
		},
	}).Parse(`{{capture $}}{{fail}}`)))

	h := Handler(jw, "handlerfail", tag.Tag("ok"))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	logged := logger.sync(t, jw)
	if len(logged) != 1 || !errors.Is(logged[0], renderErr) {
		t.Fatalf("logged errors = %#v, want %v", logged, renderErr)
	}
	if captured == nil {
		t.Fatal("handler template did not expose its request")
	}
	if leaked := captured.GetElementByJid(1); leaked != nil {
		t.Fatalf("expected failed render element to be removed from request registry: %v", leaked.Jid())
	}
}

func TestHandler_TemplateWritesKeepPendingRequestFresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, err := jaws.New()
		if err != nil {
			t.Fatal(err)
		}
		jw.MaxPendingRequestsPerIP = 1

		captured := make(chan *jaws.Request, 1)
		waitingMiddle := make(chan struct{})
		waitMiddle := make(chan struct{})
		middleWritten := make(chan struct{})
		unblock := make(chan struct{})
		var closeWaitMiddle sync.Once
		var closeUnblock sync.Once
		defer func() {
			closeWaitMiddle.Do(func() {
				close(waitMiddle)
			})
			closeUnblock.Do(func() {
				close(unblock)
			})
			jw.Close()
			synctest.Wait()
		}()

		_ = jw.AddTemplateLookuper(template.Must(template.New("slowhandler").Funcs(template.FuncMap{
			"capture": func(w With) string {
				captured <- w.RequestWriter.Request
				return ""
			},
			"waitMiddle": func() string {
				close(waitingMiddle)
				<-waitMiddle
				return ""
			},
			"pause": func() string {
				close(middleWritten)
				<-unblock
				return ""
			},
		}).Parse(`{{capture $}}prefix{{waitMiddle}}middle{{pause}}suffix`)))

		h := Handler(jw, "slowhandler", tag.Tag("ok"))
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		done := make(chan struct{})
		go func() {
			defer close(done)
			h.ServeHTTP(rr, req)
		}()

		var first *jaws.Request
		select {
		case first = <-captured:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for handler request")
		}
		select {
		case <-waitingMiddle:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for handler template to pause")
		}

		firstKey := first.JawsKeyString()
		time.Sleep(2200 * time.Millisecond)
		go jw.ServeWithTimeout(time.Second)
		synctest.Wait()
		closeWaitMiddle.Do(func() {
			close(waitMiddle)
		})
		select {
		case <-middleWritten:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for handler template write")
		}

		secondReq := httptest.NewRequest(http.MethodGet, "/second", nil)
		secondReq.RemoteAddr = req.RemoteAddr
		_ = jw.NewRequest(httptest.NewRecorder(), secondReq)
		gotKey := first.JawsKeyString()
		gotInitial := first.Initial()

		closeUnblock.Do(func() {
			close(unblock)
		})
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for handler to finish")
		}

		if gotKey != firstKey || gotInitial != req {
			t.Fatalf("in-flight handler request was recycled after template write: key %q initial %p, want key %q initial %p", gotKey, gotInitial, firstKey, req)
		}
	})
}

func TestTemplate_UpdateLogsMissingTemplate(t *testing.T) {
	jw, rq := newCoreRequest(t)
	logger := new(templateLogger)
	jw.Logger = logger

	tpl := NewTemplate("div", "missingtemplate", tag.Tag("dot"))
	elem := rq.NewElement(tpl)
	tpl.JawsUpdate(elem)

	logged := logger.sync(t, jw)
	if len(logged) != 1 {
		t.Fatalf("logged errors = %d, want 1", len(logged))
	}
	if !errors.Is(logged[0], ErrMissingTemplate) {
		t.Fatalf("logged error = %v, want %v", logged[0], ErrMissingTemplate)
	}
}

func TestTemplate_RenderMissingTemplateSkipsDotInitialHTMLAttr(t *testing.T) {
	_, rq := newCoreRequest(t)
	dot := new(templateInitialAttrDot)
	tmpl := NewTemplate("div", "missingtemplate", dot)
	elem := rq.NewElement(tmpl)

	if err := elem.JawsRender(io.Discard, nil); !errors.Is(err, ErrMissingTemplate) {
		t.Fatalf("render error = %v, want %v", err, ErrMissingTemplate)
	}
	if calls := dot.attrCalls(); len(calls) != 0 {
		t.Fatalf("missing Template invoked JawsInitialHTMLAttr for %v", calls)
	}
}

func TestNewTemplate_EmptyWrapperDefaultsToDiv(t *testing.T) {
	tpl := NewTemplate("", "partial", tag.Tag("dot"))
	if tpl.OuterHTMLTag != "div" {
		t.Fatalf("OuterHTMLTag = %q, want %q", tpl.OuterHTMLTag, "div")
	}
}

func TestTemplate_UnwrappedSkipsDotInitialHTMLAttr(t *testing.T) {
	jw, rq := newCoreRequest(t)
	if err := jw.AddTemplateLookuper(template.Must(template.New("unwrapped").Parse(`content`))); err != nil {
		t.Fatal(err)
	}

	dot := new(templateInitialAttrDot)
	_, got := renderUI(t, rq, Template{Name: "unwrapped", Dot: dot})
	if got != "content" {
		t.Fatalf("unwrapped Template rendered %q, want %q", got, "content")
	}
	if calls := dot.attrCalls(); len(calls) != 0 {
		t.Fatalf("unwrapped Template invoked JawsInitialHTMLAttr for %v", calls)
	}
}

// TestTemplate_UpdateLogsExecuteError needs a template whose initial render succeeds and
// whose later update fails: a wrapped Template updates only an Element it rendered, so a
// template that always fails could never establish the state slot to begin with.
func TestTemplate_UpdateLogsExecuteError(t *testing.T) {
	logger := new(templateLogger)
	jw, rq := newConfiguredCoreRequest(t, withLogger(logger))
	if err := jw.AddTemplateLookuper(template.Must(template.New("badupdate").Parse(
		`{{$.Dot.Check}}`,
	))); err != nil {
		t.Fatal(err)
	}

	dot := &ownedDot{}
	tpl := NewTemplate("div", "badupdate", dot)
	elem := rq.NewElement(tpl)
	if err := elem.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}

	dot.setFail(errOwnedDotCheck)
	tpl.JawsUpdate(elem)

	logged := logger.sync(t, jw)
	if len(logged) != 1 {
		t.Fatalf("logged errors = %d, want 1", len(logged))
	}
	if !errors.Is(logged[0], errOwnedDotCheck) {
		t.Fatalf("logged error = %v, want %v", logged[0], errOwnedDotCheck)
	}
}

func TestTemplate_UpdateLogsUnclaimedState(t *testing.T) {
	logger := new(templateLogger)
	jw, rq := newConfiguredCoreRequest(t, withLogger(logger))
	if err := jw.AddTemplateLookuper(template.Must(template.New("unclaimed").Parse(`ok`))); err != nil {
		t.Fatal(err)
	}

	// A wrapped Template has nothing to reconcile against on an Element it never
	// rendered, so it executes nothing and reports it. This is a defensive diagnostic,
	// not a supported lifecycle.
	tpl := NewTemplate("div", "unclaimed", tag.Tag("dot"))
	tpl.JawsUpdate(rq.NewElement(tpl))

	logged := logger.sync(t, jw)
	if len(logged) != 1 || !errors.Is(logged[0], ErrElementStateUnclaimed) {
		t.Fatalf("logged errors = %v, want one %v", logged, ErrElementStateUnclaimed)
	}
}

func TestPageTemplate_UpdateNoop(t *testing.T) {
	(&pageTemplate{}).JawsUpdate(nil)
}

func TestTemplate_RenderReturnsTagExpandError(t *testing.T) {
	jw, rq := newCoreRequest(t)
	_ = jw.AddTemplateLookuper(template.Must(template.New("uitempl").Parse(
		`{{with $.Dot}}<span>{{.}}</span>{{end}}`,
	)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}
	if err := rw.Template("div", "uitempl", "plain-string-dot"); err == nil {
		t.Fatal("expected tag expansion error")
	}
}

func TestTemplate_RenderClosesWrapperOnExecuteError(t *testing.T) {
	jw, rq := newCoreRequest(t)
	_ = jw.AddTemplateLookuper(template.Must(template.New("badrender").Parse(
		`<span>{{$.Dot.MissingField}}</span>`,
	)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}
	err := rw.Template("div", "badrender", &templateUpdateDot{})
	if err == nil {
		t.Fatal("expected execute error")
	}
	if !strings.Contains(err.Error(), "MissingField") {
		t.Fatalf("err = %v, want MissingField error", err)
	}
	// The wrapper start tag is flushed before execute runs; on execute failure the
	// closing tag must still be emitted so the streamed output stays balanced.
	out := sb.String()
	if !strings.HasPrefix(out, "<div") {
		t.Fatalf("output missing wrapper start tag: %q", out)
	}
	if !strings.HasSuffix(out, "</div>") {
		t.Fatalf("wrapper not closed on execute error: %q", out)
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

	_ = jw.AddTemplateLookuper(template.Must(template.New("update").Parse(
		`{{with $.Dot}}<span>{{.Text}}</span>{{end}}`,
	)))

	go jw.Serve()
	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	defer tr.Close()
	<-tr.ReadyCh

	dot := &templateUpdateDot{Text: "before"}
	tpl := NewTemplate("div", "update", dot)
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
