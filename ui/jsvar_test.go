package ui

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"sync"
	"testing"

	pkg "github.com/linkdata/jaws/jaws"
	"github.com/linkdata/jaws/what"
)

type jsVarData struct {
	Text string  `json:"text"`
	Num  float64 `json:"num"`
}

type jsVarPathHooks struct {
	Value       string `json:"value"`
	setCalls    int
	pathSetCall int
}

func (d *jsVarPathHooks) JawsSetPath(_ *pkg.Element, _ string, v any) error {
	s := fmt.Sprint(v)
	if d.Value == s {
		return pkg.ErrValueUnchanged
	}
	d.Value = s
	d.setCalls++
	return nil
}

func (d *jsVarPathHooks) JawsPathSet(*pkg.Element, string, any) {
	d.pathSetCall++
}

type testJsVarMaker struct{}

func (testJsVarMaker) JawsMakeJsVar(*pkg.Request) (IsJsVar, error) {
	var mu sync.Mutex
	v := jsVarData{Text: "maker", Num: 1}
	return NewJsVar(&mu, &v), nil
}

type errorJsVarMaker struct{}

func (errorJsVarMaker) JawsMakeJsVar(*pkg.Request) (IsJsVar, error) {
	return nil, errors.New("maker error")
}

func TestJsVar_RenderSetAndEvent(t *testing.T) {
	jw, rq := newRequest(t)
	go jw.Serve()

	var mu sync.Mutex
	v := jsVarData{Text: "quote(')", Num: 1}
	jsv := NewJsVar(&mu, &v)
	elem := rq.NewElement(jsv)

	var sb bytes.Buffer
	if err := jsv.JawsRender(elem, &sb, []any{"myjsvar", template.HTMLAttr(`data-x="1"`)}); err != nil {
		t.Fatal(err)
	}
	got := sb.String()
	if !strings.Contains(got, `data-jawsname="myjsvar"`) ||
		!strings.Contains(got, `\u0027`) ||
		!strings.Contains(got, `data-x="1"`) {
		t.Fatalf("unexpected jsvar render: %q", got)
	}

	if jsv.JawsGetTag(rq) == nil {
		t.Fatal("expected non-nil tag after render")
	}
	if gotV := jsv.JawsGet(nil); gotV.Text != "quote(')" || gotV.Num != 1 {
		t.Fatalf("unexpected value %#v", gotV)
	}
	if gotPath := jsv.JawsGetPath(nil, "text"); gotPath != "quote(')" {
		t.Fatalf("unexpected path value %#v", gotPath)
	}
	_ = jsv.JawsGetPath(elem, "[")
	jsv.JawsUpdate(elem)

	if err := jsv.JawsSetPath(elem, "text", "new"); err != nil {
		t.Fatal(err)
	}
	if err := jsv.JawsSetPath(elem, "text", "new"); !errors.Is(err, pkg.ErrValueUnchanged) {
		t.Fatalf("expected ErrValueUnchanged, got %v", err)
	}
	if err := jsv.JawsSet(elem, jsVarData{Text: "obj", Num: 2}); err != nil {
		t.Fatal(err)
	}

	if err := jsv.JawsEvent(elem, what.Set, `text="evt"`); err != nil {
		t.Fatal(err)
	}
	if v.Text != "evt" {
		t.Fatalf("expected updated value, got %#v", v)
	}
	if err := jsv.JawsEvent(elem, what.Set, `text="evt"`); err != nil {
		t.Fatalf("expected unchanged error elided, got %v", err)
	}
	if err := jsv.JawsEvent(elem, what.Set, `text=`); err == nil {
		t.Fatal("expected unmarshal error")
	}
	if err := jsv.JawsEvent(elem, what.Set, `badpayload`); !errors.Is(err, pkg.ErrEventUnhandled) {
		t.Fatalf("expected ErrEventUnhandled, got %v", err)
	}
	if err := jsv.JawsEvent(elem, what.Click, `text="x"`); !errors.Is(err, pkg.ErrEventUnhandled) {
		t.Fatalf("expected ErrEventUnhandled, got %v", err)
	}

	if err := elideErrValueUnchanged(pkg.ErrValueUnchanged); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	other := errors.New("other")
	if err := elideErrValueUnchanged(other); !errors.Is(err, other) {
		t.Fatalf("expected passthrough error, got %v", err)
	}
}

func TestJsVar_PathHooksAndRequestWriter(t *testing.T) {
	jw, rq := newRequest(t)
	go jw.Serve()

	var mu sync.Mutex
	v := jsVarPathHooks{Value: "a"}
	jsv := NewJsVar(&mu, &v)
	elem := rq.NewElement(jsv)

	var sb bytes.Buffer
	if err := jsv.JawsRender(elem, &sb, []any{"pvar"}); err != nil {
		t.Fatal(err)
	}
	if err := jsv.setPathLock(elem, "value", "b"); err != nil {
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
	if err := jsv.JawsSetPath(elem, "value", "c"); !errors.Is(err, pkg.ErrValueUnchanged) {
		t.Fatalf("expected ErrValueUnchanged, got %v", err)
	}

	if got := string(appendAttrs(nil, []template.HTMLAttr{"x", "", "y"})); got != " x y" {
		t.Fatalf("unexpected attrs %q", got)
	}

	var rwmu sync.RWMutex
	jsvRW := NewJsVar(&rwmu, &v)
	if _, ok := jsvRW.RWLocker.(*sync.RWMutex); !ok {
		t.Fatalf("expected RWMutex locker, got %T", jsvRW.RWLocker)
	}
	var plainMu sync.Mutex
	jsvPlain := NewJsVar(&plainMu, &v)
	if _, ok := jsvPlain.RWLocker.(rwlocker); !ok {
		t.Fatalf("expected rwlocker wrapper, got %T", jsvPlain.RWLocker)
	}

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
	if got := sb.String(); !strings.Contains(got, `data-jawsname="direct"`) || !strings.Contains(got, `data-jawsname="maker"`) {
		t.Fatalf("unexpected jsvar output %q", got)
	}
}
