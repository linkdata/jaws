package ui

import (
	"bytes"
	"errors"
	"sync"
	"testing"

	"github.com/linkdata/jaws"
)

func TestJsVarBindingContainerRollbackRestoresRoute(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	value := 1
	store := newTestJsVarStore(t, jw, "client", &mu, &value)
	store.ClientCheck = func(*jaws.Element, *int, string) error { return nil }
	oldBinding, oldElem, _ := renderTestJsVar(t, rq, store)
	children := &testContainer{contents: []jaws.UI{store.Bind()}}
	container := rq.NewElement(NewContainer("div", children))
	if err := container.JawsRender(&bytes.Buffer{}, nil); !errors.Is(err, ErrJsVarNameConflict) {
		t.Fatalf("container render error = %v, want name conflict", err)
	}
	if got := rq.GetElements(store); len(got) != 1 || got[0] != oldElem {
		t.Fatalf("bindings after rollback = %v, want original", got)
	}
	if err := oldBinding.JawsInput(oldElem, "=2"); err != nil || value != 2 {
		t.Fatalf("restored binding input: value=%d err=%v", value, err)
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
