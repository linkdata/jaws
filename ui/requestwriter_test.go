package ui

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws/core"
)

type testRWUpdater struct {
	called int
}

func (u *testRWUpdater) JawsUpdate(*core.Element) {
	u.called++
}

func newSessionBoundRequest(t *testing.T) (*core.Jaws, *core.Request) {
	t.Helper()
	jw, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	if sess := jw.NewSession(rr, hr); sess == nil {
		t.Fatal("expected session")
	}
	rq := jw.NewRequest(hr)
	if rq == nil {
		t.Fatal("expected request")
	}
	return jw, rq
}

func TestRequestWriter_MethodsAndWidgetHelpers(t *testing.T) {
	jw, rq := newSessionBoundRequest(t)
	var buf bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &buf}

	if _, err := rw.Write([]byte("prefix")); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "prefix" {
		t.Fatalf("unexpected write output %q", got)
	}
	if !rq.Rendering.Load() {
		t.Fatal("expected rendering=true after write")
	}

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

	if err := rw.UI(NewSpan(testHTMLGetter("ui"))); err != nil {
		t.Fatal(err)
	}

	tc := &testContainer{contents: []core.UI{NewSpan(testHTMLGetter("in"))}}
	date := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	sh := &testSelectHandler{
		testContainer: &testContainer{contents: []core.UI{NewOption(core.NewNamedBool(nil, "x", "X", true))}},
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

	var mu sync.Mutex
	l := rwlocker{Locker: &mu}
	l.RLock()
	l.RUnlock()
}
