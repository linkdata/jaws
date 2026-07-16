package ui

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/named"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type testRWUpdater struct {
	called int
}

func (u *testRWUpdater) JawsUpdate(elem *jaws.Element) {
	u.called++
}

type registerClickUpdater struct {
	testRWUpdater
	clicks int
}

type registerRendererUpdater struct {
	param    tag.Tag
	rendered bool
	updated  bool
	sawParam bool
}

func (u *registerRendererUpdater) JawsRender(*jaws.Element, io.Writer, []any) error {
	u.rendered = true
	return errors.New("register must not render its updater")
}

func (u *registerRendererUpdater) JawsUpdate(elem *jaws.Element) {
	u.updated = true
	u.sawParam = elem.HasTag(u.param)
}

type nonComparableStringSetter []string

func (s nonComparableStringSetter) JawsGet(*jaws.Element) string {
	return s[0]
}

func (s nonComparableStringSetter) JawsSet(_ *jaws.Element, value string) error {
	if s[0] == value {
		return jaws.ErrValueUnchanged
	}
	s[0] = value
	return nil
}

func (u *registerClickUpdater) JawsClick(elem *jaws.Element, click jaws.Click) error {
	u.clicks++
	return nil
}

type requestWriterFailGetter struct {
	err error
}

func (g requestWriterFailGetter) JawsGetHTML(elem *jaws.Element) template.HTML { return "x" }
func (g requestWriterFailGetter) JawsGetTag(tag.Context) any                   { return g }
func (g requestWriterFailGetter) JawsInit(elem *jaws.Element) error            { return g.err }

func TestRequestWriter_MethodsAndWidgetHelpers(t *testing.T) {
	jw, rq := newCoreSessionBoundRequest(t)
	var buf bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &buf}

	if _, err := rw.Write([]byte("prefix")); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "prefix" {
		t.Fatalf("unexpected write output %q", got)
	}
	// Write records the current second via Request.MarkWritten (covered by the core
	// package's pending-eviction tests); lastWriteSeconds is unexported here.

	if rw.Initial() == nil {
		t.Fatal("expected initial request")
	}
	if rw.Session() == nil {
		t.Fatal("expected session")
	}
	rw.Set("k", "v")
	if got := rw.Get("k"); got != "v" {
		t.Fatalf("unexpected session value %#v", got)
	}

	if err := rw.HeadHTML(); err != nil {
		t.Fatal(err)
	}
	if err := rw.TailHTML(); err != nil {
		t.Fatal(err)
	}

	if err := rw.NewUI(NewSpan(testHTMLGetter("ui"))); err != nil {
		t.Fatal(err)
	}

	tc := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("in"))}}
	date := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	sh := &testSelectHandler{
		testContainer: &testContainer{contents: []jaws.UI{NewOption(named.NewBool(nil, "x", "X", true))}},
		testSetter:    newTestSetter("x"),
	}

	calls := []func() error{
		func() error { return rw.A("a") },
		func() error { return rw.Button("b") },
		func() error { return rw.Checkbox(true) },
		func() error { return rw.Container("section", tc) },
		func() error { return rw.Date(date) },
		func() error { return rw.Div("d") },
		func() error { return rw.Img("img.png") },
		func() error { return rw.Label("l") },
		func() error { return rw.Li("li") },
		func() error { return rw.Number(1.2) },
		func() error { return rw.Password("pw") },
		func() error { return rw.Radio(false) },
		func() error { return rw.Range(2.3) },
		func() error { return rw.Select(sh) },
		func() error { return rw.Span("sp") },
		func() error { return rw.Tbody(tc) },
		func() error { return rw.Td("td") },
		func() error { return rw.Text("txt") },
		func() error { return rw.Textarea("ta") },
		func() error { return rw.Tr("tr") },
	}
	for i, call := range calls {
		if err := call(); err != nil {
			t.Fatalf("helper[%d] failed: %v", i, err)
		}
	}

	up := &testRWUpdater{}
	id := rw.Register(up)
	if !id.IsValid() {
		t.Fatalf("invalid register id %v", id)
	}
	if up.called != 1 {
		t.Fatalf("expected updater called once, got %d", up.called)
	}

	got := buf.String()
	if !strings.Contains(got, `<a id="Jid.`) ||
		!strings.Contains(got, `<button id="Jid.`) ||
		!strings.Contains(got, `<input id="Jid.`) ||
		!strings.Contains(got, `<textarea id="Jid.`) ||
		!strings.Contains(got, `<tbody id="Jid.`) {
		t.Fatalf("missing expected rendered widgets: %q", got)
	}

	// Keep the request live until end of test to avoid races with async cleanup.
	_ = jw
}

func TestErrMissingTemplateAndRWLocker(t *testing.T) {
	err := errMissingTemplate("missing")
	if got := err.Error(); got != `missing template "missing"` {
		t.Fatalf("unexpected error text %q", got)
	}
	if !errors.Is(err, ErrMissingTemplate) {
		t.Fatal("expected errors.Is match")
	}
}

func TestRequestWriterUI_RenderErrorDoesNotLeakElement(t *testing.T) {
	_, rq := newCoreRequest(t)
	var buf bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &buf}

	renderErr := errors.New("render failed")
	if err := rw.NewUI(NewA(requestWriterFailGetter{err: renderErr})); !errors.Is(err, renderErr) {
		t.Fatalf("want %v got %v", renderErr, err)
	}

	if leaked := rq.GetElementByJid(1); leaked != nil {
		t.Fatalf("expected failed render element to be removed from request registry: %v", leaked.Jid())
	}
}

func TestRequestWriter_RegisterFreezesElement(t *testing.T) {
	_, rq := newCoreRequest(t)
	// A production server has a Logger configured; with one, the rejected late
	// handler is reported via MustLog instead of panicking.
	rq.Jaws.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	var buf bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &buf}

	up := &testRWUpdater{}
	id := rw.Register(up)
	if up.called != 1 {
		t.Fatalf("expected updater called once, got %d", up.called)
	}
	elem := rq.GetElementByJid(id)
	if elem == nil {
		t.Fatal("expected registered element to be retained")
	}

	// Register freezes the element after its initial setup, so adding handlers
	// afterwards is rejected: debug panics, production reports via MustLog and drops.
	// (Without the Freeze, a never-rendered element would never trip the guard.)
	if deadlock.Debug {
		defer func() {
			if recover() == nil {
				t.Error("expected panic adding handlers to a frozen registered element")
			}
		}()
		elem.AddHandlers(struct{}{})
		t.Error("expected panic adding handlers to a frozen registered element")
		return
	}
	elem.AddHandlers(struct{}{}) // production logs and drops, must not panic
}

func TestRequestWriter_RegisterUsesUpdaterEventHandler(t *testing.T) {
	_, rq := newCoreRequest(t)
	var buf bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &buf}

	up := &registerClickUpdater{}
	id := rw.Register(up)
	elem := rq.GetElementByJid(id)
	if elem == nil {
		t.Fatal("expected registered element to be retained")
	}

	if err := jaws.CallEventHandlers(elem.UI(), elem, what.Click, "1 2 0 registered"); err != nil {
		t.Fatal(err)
	}
	if up.clicks != 1 {
		t.Fatalf("expected updater click handler to be called once, got %d", up.clicks)
	}
}

func TestRequestWriter_RegisterDoesNotRenderUpdater(t *testing.T) {
	_, rq := newCoreRequest(t)
	rw := RequestWriter{Request: rq, Writer: io.Discard}
	up := &registerRendererUpdater{param: tag.Tag("param")}

	id := rw.Register(up, up.param)
	if !id.IsValid() {
		t.Fatalf("Register returned invalid id %v", id)
	}
	if !up.updated {
		t.Fatal("expected Register to run the initial update")
	}
	if !up.sawParam {
		t.Fatal("initial update did not observe the parameter tag")
	}
	if up.rendered {
		t.Fatal("Register rendered an update-only updater")
	}
}

func TestRequestWriter_RegisterInitializesStandardDirtyTags(t *testing.T) {
	_, rq := newCoreRequest(t)
	rw := RequestWriter{Request: rq, Writer: io.Discard}

	var textMu sync.Mutex
	textValue := "text"
	text := NewText(bind.New(&textMu, &textValue))
	boolSetter := newTestSetter(true)
	checkbox := NewCheckbox(boolSetter)
	floatSetter := newTestSetter(1.5)
	number := NewNumber(floatSetter)
	dateSetter := newTestSetter(time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC))
	date := NewDate(dateSetter)
	selectHandler := &testSelectHandler{
		testContainer: &testContainer{},
		testSetter:    newTestSetter(""),
	}
	selectUI := NewSelect(selectHandler)

	tests := []struct {
		name    string
		updater jaws.Updater
		gotTag  func() any
		wantTag any
	}{
		{name: "text", updater: text, gotTag: func() any { return text.tag }, wantTag: &textValue},
		{name: "checkbox", updater: checkbox, gotTag: func() any { return checkbox.tag }, wantTag: boolSetter},
		{name: "number", updater: number, gotTag: func() any { return number.tag }, wantTag: floatSetter},
		{name: "date", updater: date, gotTag: func() any { return date.tag }, wantTag: dateSetter},
		{name: "select", updater: selectUI, gotTag: func() any { return selectUI.tag }, wantTag: selectHandler},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := rw.Register(tt.updater)
			elem := rq.GetElementByJid(id)
			if elem == nil {
				t.Fatalf("Register(%s) did not retain its Element", tt.name)
			}
			if got := tt.gotTag(); got != tt.wantTag {
				t.Fatalf("dirty tag = %#v, want %#v", got, tt.wantTag)
			}
			if !elem.HasTag(tt.wantTag) {
				t.Fatalf("registered Element missing dirty tag %#v", tt.wantTag)
			}
		})
	}
}

func TestRequestWriter_RegisterDeclinesNonComparableDirtyTag(t *testing.T) {
	_, rq := newCoreRequest(t)
	rw := RequestWriter{Request: rq, Writer: io.Discard}
	setter := nonComparableStringSetter{"before"}
	input := NewText(setter)

	id := rw.Register(input)
	if input.tag != nil {
		t.Fatalf("dirty tag = %#v, want nil", input.tag)
	}
	elem := rq.GetElementByJid(id)
	if elem == nil {
		t.Fatal("Register did not retain its Element")
	}
	if err := jaws.CallEventHandlers(elem.UI(), elem, what.Input, "after"); err != nil {
		t.Fatalf("registered input event returned %v", err)
	}
	if got := setter[0]; got != "after" {
		t.Fatalf("setter value = %q, want %q", got, "after")
	}
}

// TestRequestWriter_RegisterInputDirtiesSharedTag binds one setter to two views:
// a normally rendered input (A) and a Register'd input (B). An Input event to B
// must dirty the shared bound tag so A, which shares the setter, receives a
// corrective Value update. Without assigning B's dirty tag during Register, B's
// tag stays nil, the dirty mark expands to nothing and A never updates.
func TestRequestWriter_RegisterInputDirtiesSharedTag(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	defer tr.Close()
	<-tr.ReadyCh

	ss := newTestSetter("start")

	// View A: a normally rendered text input bound to ss.
	viewA := NewText(ss)
	elemA := tr.NewElement(viewA)
	var buf strings.Builder
	if err := elemA.JawsRender(&buf, nil); err != nil {
		t.Fatal(err)
	}

	// View B: the same setter registered as an update-only input widget.
	rw := RequestWriter{Request: tr.Request, Writer: tr.Recorder}
	idB := rw.Register(NewText(ss))

	// Deliver an Input event to B; it must dirty the shared bound tag.
	tr.InCh <- wire.WsMsg{Jid: idB, What: what.Input, Data: "typed"}

	deadline := time.After(time.Second)
	for {
		select {
		case msg := <-tr.OutCh:
			if msg.What == what.Value && msg.Jid == elemA.Jid() && msg.Data == "typed" {
				return
			}
		case <-deadline:
			t.Fatal("view A never received the corrective Value update; B's input did not dirty the shared tag")
		}
	}
}
