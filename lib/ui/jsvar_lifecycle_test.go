package ui

import (
	"bytes"
	"errors"
	"html/template"
	"sync"
	"testing"

	"github.com/linkdata/jaws"
)

var errJsVarLifecycleRender = errors.New("render failed")

type jsVarLifecycleDot struct {
	Store *JsVarStore[int]
	Fail  bool
}

func (dot *jsVarLifecycleDot) Check() (string, error) {
	if dot.Fail {
		return "", errJsVarLifecycleRender
	}
	return "", nil
}

func TestJsVarBindingTemplateRollbackRestoresRoute(t *testing.T) {
	logger := new(templateLogger)
	jw, rq := newConfiguredCoreRequest(t, func(jw *jaws.Jaws) { jw.Logger = logger })
	if err := jw.AddTemplateLookuper(template.Must(template.New("lifecycle").Parse(`{{define "lifecycle"}}{{$.NewUI ($.Dot.Store.Bind)}}{{$.Dot.Check}}{{end}}`))); err != nil {
		t.Fatal(err)
	}
	var mu sync.RWMutex
	value := 1
	store := newTestJsVarStore(t, jw, "client", &mu, &value)
	store.ClientCheck = func(*jaws.Element, *int, string) error { return nil }
	dot := &jsVarLifecycleDot{Store: store}
	tmpl := NewTemplate("div", "lifecycle", dot)
	parent := rq.NewElement(tmpl)
	if err := parent.JawsRender(&bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	old := rq.GetElements(store)
	if len(old) != 1 {
		t.Fatalf("initial bindings = %d, want 1", len(old))
	}

	dot.Fail = true
	tmpl.JawsUpdate(parent)
	if logged := logger.sync(t, jw); len(logged) != 1 || !errors.Is(logged[0], errJsVarLifecycleRender) {
		t.Fatalf("logged errors = %v, want render error", logged)
	}
	if got := rq.GetElements(store); len(got) != 1 || got[0] != old[0] {
		t.Fatalf("bindings after rollback = %v, want original", got)
	}
	if err := old[0].UI().(*JsVarBinding[int]).JawsInput(old[0], "=2"); err != nil || value != 2 {
		t.Fatalf("restored binding input: value=%d err=%v", value, err)
	}
}

func TestJsVarBindingContainerRollbackRestoresRoute(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	value := 1
	store := newTestJsVarStore(t, jw, "client", &mu, &value)
	store.ClientCheck = func(*jaws.Element, *int, string) error { return nil }
	oldBinding, oldElem, _ := renderTestJsVar(t, rq, store)
	children := &testContainer{contents: []jaws.UI{store.Bind(), testRenderErrorUI{err: errJsVarLifecycleRender}}}
	container := rq.NewElement(NewContainer("div", children))
	if err := container.JawsRender(&bytes.Buffer{}, nil); !errors.Is(err, errJsVarLifecycleRender) {
		t.Fatalf("container render error = %v, want %v", err, errJsVarLifecycleRender)
	}
	if got := rq.GetElements(store); len(got) != 1 || got[0] != oldElem {
		t.Fatalf("bindings after rollback = %v, want original", got)
	}
	if err := oldBinding.JawsInput(oldElem, "=2"); err != nil || value != 2 {
		t.Fatalf("restored binding input: value=%d err=%v", value, err)
	}
}

func TestJsVarBindingRenderOrderChoosesLatestRoute(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	value := 1
	store := newTestJsVarStore(t, jw, "client", &mu, &value)
	first := store.Bind()
	second := store.Bind()
	firstElem := rq.NewElement(first)
	secondElem := rq.NewElement(second)
	for _, elem := range []*jaws.Element{secondElem, firstElem} {
		if err := elem.JawsRender(&bytes.Buffer{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if !first.isCurrent(firstElem) || second.isCurrent(secondElem) {
		t.Fatal("current route follows Element creation order instead of render order")
	}
}

func TestJsVarBindingRejectsDifferentJaws(t *testing.T) {
	jw, _ := newCoreRequest(t)
	_, rq := newCoreRequest(t)
	var mu sync.RWMutex
	value := 1
	store := newTestJsVarStore(t, jw, "client", &mu, &value)
	elem := rq.NewElement(store.Bind())
	var output bytes.Buffer
	if err := elem.JawsRender(&output, nil); err == nil {
		t.Fatal("binding rendered in a different Jaws instance")
	}
	if output.Len() != 0 || len(rq.GetElements(store)) != 0 {
		t.Fatalf("foreign binding output=%q tags=%v", output.String(), rq.GetElements(store))
	}
}
