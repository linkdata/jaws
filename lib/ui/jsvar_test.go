package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
)

type jsVarData struct {
	Text string  `json:"text"`
	Num  float64 `json:"num"`
}

type jsVarNilData struct {
	Value string `json:"value"`
}

type jsVarPathHooks struct {
	Value       string `json:"value"`
	setCalls    int
	pathSetCall int
}

func (d *jsVarPathHooks) JawsSetPath(elem *jaws.Element, _ string, value any) error {
	s := fmt.Sprint(value)
	if d.Value == s {
		return jaws.ErrValueUnchanged
	}
	d.Value = s
	d.setCalls++
	return nil
}

func (d *jsVarPathHooks) JawsPathSet(elem *jaws.Element, jsPath string, value any) {
	d.pathSetCall++
}

type testJsVarMaker struct{}

func (testJsVarMaker) JawsMakeJsVar(rq *jaws.Request) (IsJsVar, error) {
	var mu sync.Mutex
	v := jsVarData{Text: "maker", Num: 1}
	return NewJsVar(&mu, &v), nil
}

type errorJsVarMaker struct{}

func (errorJsVarMaker) JawsMakeJsVar(rq *jaws.Request) (IsJsVar, error) {
	return nil, errors.New("maker error")
}

func htmlAttrValue(t *testing.T, htmlText, name string) string {
	t.Helper()
	_, after, ok := strings.Cut(htmlText, name+`="`)
	if !ok {
		t.Fatalf("missing %s attr in %q", name, htmlText)
	}
	value, _, ok := strings.Cut(after, `"`)
	if !ok {
		t.Fatalf("unterminated %s attr in %q", name, htmlText)
	}
	return html.UnescapeString(value)
}

func TestJsVar_RenderSetAndEvent(t *testing.T) {
	jw, rq := newCoreRequest(t)
	go jw.Serve()

	var mu sync.Mutex
	v := jsVarData{Text: `quote(') "& <script>`, Num: 1}
	jsv := NewJsVar(&mu, &v)
	elem := rq.NewElement(jsv)

	var sb bytes.Buffer
	if err := jsv.JawsRender(elem, &sb, []any{"myjsvar", template.HTMLAttr(`data-x="1"`)}); err != nil {
		t.Fatal(err)
	}
	got := sb.String()
	if !strings.Contains(got, `data-jawsname="myjsvar"`) ||
		!strings.Contains(got, `data-x="1"`) {
		t.Fatalf("unexpected jsvar render: %q", got)
	}
	if strings.Contains(got, `data-jawsdata='`) {
		t.Fatalf("jsvar render used single-quoted data attr: %q", got)
	}
	var gotData jsVarData
	if err := json.Unmarshal([]byte(htmlAttrValue(t, got, "data-jawsdata")), &gotData); err != nil {
		t.Fatalf("data-jawsdata is not JSON: %v", err)
	}
	if gotData != v {
		t.Fatalf("data-jawsdata = %#v, want %#v", gotData, v)
	}

	if jsv.JawsGetTag(rq) == nil {
		t.Fatal("expected non-nil tag after render")
	}
	if gotV := jsv.JawsGet(nil); gotV.Text != v.Text || gotV.Num != 1 {
		t.Fatalf("unexpected value %#v", gotV)
	}
	if gotPath := jsv.JawsGetPath(nil, "text"); gotPath != v.Text {
		t.Fatalf("unexpected path value %#v", gotPath)
	}
	_ = jsv.JawsGetPath(elem, "[")
	jsv.JawsUpdate(elem)

	if err := jsv.JawsSetPath(elem, "text", "new"); err != nil {
		t.Fatal(err)
	}
	if err := jsv.JawsSetPath(elem, "text", "new"); !errors.Is(err, jaws.ErrValueUnchanged) {
		t.Fatalf("expected ErrValueUnchanged, got %v", err)
	}
	if err := jsv.JawsSet(elem, jsVarData{Text: "obj", Num: 2}); err != nil {
		t.Fatal(err)
	}

	if err := jaws.CallEventHandlers(jsv, elem, what.Set, `text="evt"`); err != nil {
		t.Fatal(err)
	}
	if v.Text != "evt" {
		t.Fatalf("expected updated value, got %#v", v)
	}
	if err := jaws.CallEventHandlers(jsv, elem, what.Set, `text="evt"`); err != nil {
		t.Fatalf("expected unchanged error elided, got %v", err)
	}
	if err := jaws.CallEventHandlers(jsv, elem, what.Set, `text=`); err == nil {
		t.Fatal("expected unmarshal error")
	}
	if err := jaws.CallEventHandlers(jsv, elem, what.Set, `badpayload`); !errors.Is(err, jaws.ErrEventUnhandled) {
		t.Fatalf("expected ErrEventUnhandled, got %v", err)
	}
	if err := jaws.CallEventHandlers(jsv, elem, what.Click, `1 2 0 x`); !errors.Is(err, jaws.ErrEventUnhandled) {
		t.Fatalf("expected ErrEventUnhandled, got %v", err)
	}

}

// TestJsVar_SetBroadcastsWirePayload pins the wire payload broadcast when a
// JsVar path is set: subscribed peers receive a what.Set carrying "path=<json
// value>". This guards the documented "broadcast carries the caller's JSON
// value" contract, which line coverage alone would not catch.
func TestJsVar_SetBroadcastsWirePayload(t *testing.T) {
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

	var mu sync.Mutex
	v := jsVarData{Text: "old"}
	jsv := NewJsVar(&mu, &v)
	elem := tr.NewElement(jsv)
	var sb strings.Builder
	if err := jsv.JawsRender(elem, &sb, []any{"v"}); err != nil {
		t.Fatal(err)
	}

	if err := jsv.JawsSetPath(elem, "text", "new"); err != nil {
		t.Fatal(err)
	}

	select {
	case <-t.Context().Done():
		t.Fatal("no Set broadcast received")
	case msg := <-tr.OutCh:
		if msg.What != what.Set || msg.Data != `text="new"` {
			t.Fatalf("broadcast = {%v %q}, want {Set `text=\"new\"`}", msg.What, msg.Data)
		}
	}
}

// TestJsVar_SetBeforeRenderDoesNotBroadcast verifies that a JawsSet/JawsSetPath
// on a JsVar that has not yet been rendered produces no broadcast. Before the
// first render the dirty tag is nil, and a what.Set with a nil Dest would target
// every live element; the guard suppresses it. A second JsVar is rendered first
// so there is a live subscription that would receive such a stray nil-Dest frame,
// proving the guard is load-bearing rather than masked by an empty request.
func TestJsVar_SetBeforeRenderDoesNotBroadcast(t *testing.T) {
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

	// A rendered JsVar provides a live element that a stray nil-Dest frame reaches.
	var watchedMu sync.Mutex
	watched := jsVarData{Text: "watched"}
	jsvWatched := NewJsVar(&watchedMu, &watched)
	elemWatched := tr.NewElement(jsvWatched)
	var sb strings.Builder
	if err := jsvWatched.JawsRender(elemWatched, &sb, []any{"watched"}); err != nil {
		t.Fatal(err)
	}

	// This JsVar is never rendered, so its dirty tag stays nil.
	var mu sync.Mutex
	v := jsVarData{Text: "old"}
	jsv := NewJsVar(&mu, &v)
	elem := tr.NewElement(jsv)

	if err := jsv.JawsSetPath(elem, "text", "new"); err != nil {
		t.Fatal(err)
	}
	if err := jsv.JawsSet(elem, jsVarData{Text: "root"}); err != nil {
		t.Fatal(err)
	}
	// The set must still mutate the bound value, just not broadcast.
	if jsv.JawsGet(elem).Text != "root" {
		t.Fatalf("pre-render set did not apply, got %#v", jsv.JawsGet(elem))
	}

	select {
	case msg := <-tr.OutCh:
		t.Fatalf("pre-render set broadcast a frame %#v, want none", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestJsVar_SetBeforeRenderDoesNotNotifySetPather(t *testing.T) {
	_, rq := newCoreRequest(t)

	var mu sync.Mutex
	v := jsVarPathHooks{Value: "old"}
	jsv := NewJsVar(&mu, &v)
	elem := rq.NewElement(jsv)

	if err := jsv.JawsSetPath(elem, "value", "new"); err != nil {
		t.Fatal(err)
	}
	if v.Value != "new" {
		t.Fatalf("pre-render set did not apply, got %#v", v)
	}
	if v.pathSetCall != 0 {
		t.Fatalf("pre-render set called JawsPathSet %d times, want 0 because no broadcast was queued", v.pathSetCall)
	}
}

// TestJsVar_RejectsProtocolBytesInPath verifies that a JsVar path containing a
// byte significant to the browser protocol is rejected before it is applied or
// broadcast. The path is written verbatim into a what.Set frame (only the value
// is JSON-encoded) and the client splits frames on '\n', fields on '\t', and the
// JsVar payload at the first '='. A map-backed JsVar is used so that, absent the
// guard, the reserved-byte key would set and broadcast successfully, proving the
// guard is load-bearing, not incidentally masked by a path lookup that would fail
// anyway (as it would for a struct field).
func TestJsVar_RejectsProtocolBytesInPath(t *testing.T) {
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

	var mu sync.Mutex
	// Pre-populate the keys so jq.Set would find and change them absent the guard
	// (jq does not create missing map keys), making the guard demonstrably
	// load-bearing rather than incidentally masked by a lookup that fails anyway.
	v := map[string]int{"a\tb": 1, "a\nx": 1, "a\rb": 1, "a=b": 1, "ok": 0}
	jsv := NewJsVar(&mu, &v)
	elem := tr.NewElement(jsv)
	var sb strings.Builder
	if err := jsv.JawsRender(elem, &sb, []any{"m"}); err != nil {
		t.Fatal(err)
	}

	for _, bad := range []string{"a\tb", "a\nx", "a\rb", "a=b"} {
		if err := jsv.JawsSetPath(elem, bad, 99); !errors.Is(err, ErrIllegalJsVarPath) {
			t.Fatalf("JawsSetPath(%q): expected ErrIllegalJsVarPath, got %v", bad, err)
		}
		// Rejected before apply: the pre-existing value is untouched.
		mu.Lock()
		got := v[bad]
		mu.Unlock()
		if got != 1 {
			t.Fatalf("JawsSetPath(%q): value changed to %d, expected rejection before apply", bad, got)
		}
	}

	// The incoming-browser path (JawsInput via what.Set) is likewise rejected.
	if err := jaws.CallEventHandlers(jsv, elem, what.Set, "a\tb=99"); !errors.Is(err, ErrIllegalJsVarPath) {
		t.Fatalf("JawsInput with tab path: expected ErrIllegalJsVarPath, got %v", err)
	}

	// A legitimate path still sets and broadcasts a well-formed frame.
	if err := jsv.JawsSetPath(elem, "ok", 7); err != nil {
		t.Fatal(err)
	}
	select {
	case <-t.Context().Done():
		t.Fatal("no Set broadcast received for the legal path")
	case msg := <-tr.OutCh:
		if msg.What != what.Set || msg.Data != `ok=7` {
			t.Fatalf("broadcast = {%v %q}, want {Set `ok=7`}", msg.What, msg.Data)
		}
		if strings.ContainsAny(msg.Data, "\t\n\r") || strings.Count(msg.Data, "=") != 1 {
			t.Fatalf("broadcast Data contains illegal protocol bytes: %q", msg.Data)
		}
	}
}

func TestJsVar_PathHooksAndRequestWriter(t *testing.T) {
	jw, rq := newCoreRequest(t)
	go jw.Serve()

	var mu sync.Mutex
	v := jsVarPathHooks{Value: "a"}
	jsv := NewJsVar(&mu, &v)
	elem := rq.NewElement(jsv)

	var sb bytes.Buffer
	if err := jsv.JawsRender(elem, &sb, []any{"pvar"}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := jsv.setPathLock(elem, "value", "b", false); err != nil {
		t.Fatal(err)
	}
	if v.Value != "b" || v.setCalls == 0 {
		t.Fatalf("expected path hooks to run, got %#v", v)
	}
	if err := jsv.JawsSetPath(elem, "value", "c"); err != nil {
		t.Fatal(err)
	}
	if v.pathSetCall == 0 {
		t.Fatalf("expected JawsPathSet callback, got %#v", v)
	}
	if err := jsv.JawsSetPath(elem, "value", "c"); !errors.Is(err, jaws.ErrValueUnchanged) {
		t.Fatalf("expected ErrValueUnchanged, got %v", err)
	}

	var rwmu sync.RWMutex
	jsvRW := NewJsVar(&rwmu, &v)
	if _, ok := jsvRW.RWLocker.(*sync.RWMutex); !ok {
		t.Fatalf("expected RWMutex locker, got %T", jsvRW.RWLocker)
	}
	var plainMu sync.Mutex
	jsvPlain := NewJsVar(&plainMu, &v)
	// A plain sync.Mutex does not implement bind.RWLocker, so it must have been
	// wrapped; exercise the read lock to confirm the wrapper is functional.
	jsvPlain.RWLocker.RLock()
	_ = *jsvPlain.Ptr
	jsvPlain.RWLocker.RUnlock()

	rw := RequestWriter{Request: rq, Writer: &sb}
	if err := rw.JsVar("direct", jsv); err != nil {
		t.Fatal(err)
	}
	if err := rw.JsVar("maker", testJsVarMaker{}); err != nil {
		t.Fatal(err)
	}
	if err := rw.JsVar("bad", errorJsVarMaker{}); err == nil || err.Error() != "maker error" {
		t.Fatalf("expected maker error, got %v", err)
	}
	if err := rw.JsVar("bad.name", jsv); !errors.Is(err, ErrIllegalJsVarName) {
		t.Fatalf("expected ErrIllegalJsVarName, got %v", err)
	}
	if err := rw.JsVar("badtype", 123); !errors.Is(err, ErrJsVarArgumentType) {
		t.Fatalf("expected ErrJsVarArgumentType, got %v", err)
	}
	if got := sb.String(); !strings.Contains(got, `data-jawsname="direct"`) || !strings.Contains(got, `data-jawsname="maker"`) {
		t.Fatalf("unexpected jsvar output %q", got)
	}
}

func TestRequestWriterJsVarTypedNilDoesNotPanic(t *testing.T) {
	_, rq := newCoreRequest(t)
	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}

	var nilJsVar *JsVar[int]

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("JsVar should reject typed nil values with an error, got panic: %v", r)
		}
	}()

	if err := rw.JsVar("typednil", nilJsVar); err == nil {
		t.Fatal("expected error for typed nil JsVar")
	}
}

func TestJsVar_RenderParamValidation(t *testing.T) {
	_, rq := newCoreRequest(t)

	var mu sync.Mutex
	v := jsVarData{}
	jsv := NewJsVar(&mu, &v)
	elem := rq.NewElement(jsv)

	var sb bytes.Buffer
	if err := jsv.JawsRender(elem, &sb, nil); !errors.Is(err, ErrIllegalJsVarName) {
		t.Fatalf("expected ErrIllegalJsVarName, got %v", err)
	}
	if err := jsv.JawsRender(elem, &sb, []any{123}); !errors.Is(err, ErrIllegalJsVarName) {
		t.Fatalf("expected ErrIllegalJsVarName, got %v", err)
	}
	if err := jsv.JawsRender(elem, &sb, []any{""}); !errors.Is(err, ErrIllegalJsVarName) {
		t.Fatalf("expected ErrIllegalJsVarName, got %v", err)
	}
	if err := jsv.JawsRender(elem, &sb, []any{"9bad"}); !errors.Is(err, ErrIllegalJsVarName) {
		t.Fatalf("expected ErrIllegalJsVarName, got %v", err)
	}
	if err := jsv.JawsRender(elem, &sb, []any{"bad.name"}); !errors.Is(err, ErrIllegalJsVarName) {
		t.Fatalf("expected ErrIllegalJsVarName, got %v", err)
	}
}

func TestErrIllegalJsVarName_Error(t *testing.T) {
	if got := errIllegalJsVarName("").Error(); got != "illegal jsvar name" {
		t.Fatalf("unexpected empty error string %q", got)
	}
	if got := errIllegalJsVarName("illegal syntax").Error(); got != "illegal jsvar name: illegal syntax" {
		t.Fatalf("unexpected detailed error string %q", got)
	}
}

func TestJsVar_JawsGetWithNilPointerReturnsZeroValue(t *testing.T) {
	_, rq := newCoreRequest(t)

	var mu sync.Mutex
	var ptr *jsVarNilData
	jsv := NewJsVar(&mu, ptr)
	elem := rq.NewElement(jsv)

	var sb bytes.Buffer
	if err := jsv.JawsRender(elem, &sb, []any{"nilptr"}); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("JawsGet should not panic for nil pointer: %v", r)
		}
	}()

	got := jsv.JawsGet(elem)
	if got != (jsVarNilData{}) {
		t.Fatalf("want zero value got %#v", got)
	}
}

func TestJsVar_RenderWithNilInterfaceValueDoesNotPanic(t *testing.T) {
	_, rq := newCoreRequest(t)

	var mu sync.Mutex
	var data any
	jsv := NewJsVar(&mu, &data)
	elem := rq.NewElement(jsv)

	var sb bytes.Buffer
	panicked := false
	var panicValue any
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
				panicValue = r
			}
		}()
		_ = jsv.JawsRender(elem, &sb, []any{"niliface"})
	}()
	if panicked {
		t.Fatalf("JawsRender should not panic for nil interface values: %v", panicValue)
	}
}

func TestJsVar_RenderIncludesZeroValueData(t *testing.T) {
	_, rq := newCoreRequest(t)

	var mu sync.Mutex
	v := jsVarData{}
	jsv := NewJsVar(&mu, &v)
	elem := rq.NewElement(jsv)

	var sb bytes.Buffer
	if err := jsv.JawsRender(elem, &sb, []any{"zerovar"}); err != nil {
		t.Fatal(err)
	}
	if got := sb.String(); !strings.Contains(got, `data-jawsdata="`) {
		t.Fatalf("expected data-jawsdata for zero value, got %q", got)
	}
}

// jsVarTagState has a value-receiver JawsGetTag, so *jsVarTagState also satisfies
// tag.TagGetter and calling JawsGetTag through a nil *jsVarTagState panics.
type jsVarTagState struct {
	Value string `json:"value"`
}

func (jsVarTagState) JawsGetTag(tag.Context) any {
	return tag.Tag("state")
}

// TestJsVar_RenderNilPointerWithTagGetterDoesNotLeakLock reproduces issue #196:
// rendering a JsVar bound to a nil pointer whose type implements tag.TagGetter via
// a value receiver must not invoke the getter through the typed-nil pointer, so it
// neither panics nor leaves the write lock held.
func TestJsVar_RenderNilPointerWithTagGetterDoesNotLeakLock(t *testing.T) {
	_, rq := newCoreRequest(t)

	var mu sync.Mutex
	var ptr *jsVarTagState
	jsv := NewJsVar(&mu, ptr)
	elem := rq.NewElement(jsv)

	var sb bytes.Buffer
	var panicValue any
	func() {
		defer func() { panicValue = recover() }()
		if err := jsv.JawsRender(elem, &sb, []any{"state"}); err != nil {
			t.Errorf("render returned error: %v", err)
		}
	}()
	if panicValue != nil {
		t.Errorf("render panicked: %v", panicValue)
	}
	if !mu.TryLock() {
		t.Fatal("render left the JsVar locker held")
	}
	mu.Unlock()
	// A nil Ptr omits the initial data and adds no bound-value tag.
	if got := sb.String(); strings.Contains(got, "data-jawsdata=") {
		t.Errorf("expected no data-jawsdata for nil pointer, got %q", got)
	}
}

type jsVarPanicMarshal struct{}

// MarshalJSON always panics, standing in for a bound value whose supported
// marshaling callback fails while the render write lock is held.
func (jsVarPanicMarshal) MarshalJSON() ([]byte, error) {
	panic("marshal boom")
}

// TestJsVar_RenderMarshalPanicDoesNotLeakLock verifies the render critical section
// releases the write lock even when marshaling the bound value panics.
func TestJsVar_RenderMarshalPanicDoesNotLeakLock(t *testing.T) {
	_, rq := newCoreRequest(t)

	var mu sync.Mutex
	v := jsVarPanicMarshal{}
	jsv := NewJsVar(&mu, &v)
	elem := rq.NewElement(jsv)

	var sb bytes.Buffer
	var panicValue any
	func() {
		defer func() { panicValue = recover() }()
		_ = jsv.JawsRender(elem, &sb, []any{"boom"})
	}()
	if panicValue == nil {
		t.Fatal("expected marshal panic to propagate")
	}
	if !mu.TryLock() {
		t.Fatal("render left the JsVar locker held after a marshal panic")
	}
	mu.Unlock()
}
