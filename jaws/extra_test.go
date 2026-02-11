package jaws

import (
	"io"
	"strings"
	"testing"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

type testStringTag struct{}

func (testStringTag) String() string { return "str" }

type testApplyGetterAll struct {
	initErr error
}

func (a testApplyGetterAll) JawsGetTag(*Request) any { return Tag("tg") }
func (a testApplyGetterAll) JawsClick(*Element, string) error {
	return ErrEventUnhandled
}
func (a testApplyGetterAll) JawsEvent(*Element, what.What, string) error {
	return ErrEventUnhandled
}
func (a testApplyGetterAll) JawsInit(*Element) error {
	return a.initErr
}

func TestCoverage_getterSetterFactories(t *testing.T) {
	g := MakeGetter[string]("x")
	if g.JawsGet(nil) != "x" {
		t.Fatal("unexpected getter value")
	}
	if tag := g.(TagGetter).JawsGetTag(nil); tag != nil {
		t.Fatal("expected nil tag")
	}
	// makeGetter Getter[T] passthrough branch.
	g2 := MakeGetter[string](Getter[string](getterStatic[string]{v: "y"}))
	if g2.JawsGet(nil) != "y" {
		t.Fatal("unexpected passthrough getter value")
	}

	s := MakeSetter[string]("x")
	if s.JawsGet(nil) != "x" {
		t.Fatal("unexpected setter getter value")
	}
	if err := s.JawsSet(nil, "x"); err != ErrValueNotSettable {
		t.Fatalf("unexpected err: %v", err)
	}
	// makeSetter Setter[T] passthrough branch.
	s2 := MakeSetter[string](Setter[string](setterStatic[string]{v: "z"}))
	if s2.JawsGet(nil) != "z" {
		t.Fatal("unexpected passthrough setter value")
	}
}

func TestCoverage_miscBranches(t *testing.T) {
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	// TagString pointer branch + fmt.Stringer branch
	_ = TagString(&testStringTag{})
	_ = TagString(testStringTag{})

	// Register UI render
	reg := testRegisterUI{Updater: &testUi{}}
	elem := rq.NewElement(reg)
	if err := elem.JawsRender(nil, nil); err != nil {
		t.Fatal(err)
	}

	// RequestWriter.Initial
	if rq.Request.Initial() == nil {
		t.Fatal("expected initial request")
	}
	if rq.Initial() == nil {
		t.Fatal("expected initial request from writer")
	}

	// DeleteElement exported wrapper
	e2 := rq.NewElement(&testUi{})
	id2 := e2.Jid()
	rq.DeleteElement(e2)
	if rq.GetElementByJid(id2) != nil {
		t.Fatal("expected deleted element")
	}

}

func TestCoverage_namedBoolOptionAndContains(t *testing.T) {
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	nba := NewNamedBoolArray().Add("1", "one")
	nba.Set("1", true)
	contents := nba.JawsContains(nil)
	if len(contents) != 1 {
		t.Fatalf("want 1 content got %d", len(contents))
	}
	elem := rq.NewElement(contents[0])
	var sb strings.Builder
	if err := elem.JawsRender(&sb, []any{"hidden"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "selected") {
		t.Fatal("expected selected option rendering")
	}
	contents[0].JawsUpdate(elem)
	nba.Set("1", false)
	contents[0].JawsUpdate(elem)
}

func TestCoverage_elementBranches(t *testing.T) {
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	tu := &testUi{renderFn: func(*Element, io.Writer, []any) error { return nil }}
	elem := rq.NewElement(tu)

	// renderDebug n/a branch when request lock is held.
	rq.mu.Lock()
	var sb strings.Builder
	elem.renderDebug(&sb)
	rq.mu.Unlock()
	// renderDebug tag join branch when lock is available and multiple tags exist.
	elem.Tag(Tag("a"), Tag("b"))
	sb.Reset()
	elem.renderDebug(&sb)
	if !strings.Contains(sb.String(), ", ") {
		t.Fatal("expected comma-separated tags in debug output")
	}
	// JawsRender debug path.
	rq.Jaws.Debug = true
	sb.Reset()
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}
	rq.Jaws.Debug = false

	// deleted branch in Element.JawsRender/JawsUpdate
	rq.deleteElement(elem)
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}
	elem.JawsUpdate()
}

func TestCoverage_applyGetterBranchesAndDebugNewElement(t *testing.T) {
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	elem := rq.NewElement(&testUi{})

	// nil getter
	if tag, err := elem.ApplyGetter(nil); tag != nil || err != nil {
		t.Fatalf("unexpected %v %v", tag, err)
	}

	// getter with tag/click/event/init handler
	ag := testApplyGetterAll{}
	if tag, err := elem.ApplyGetter(ag); tag != Tag("tg") || err != nil {
		t.Fatalf("unexpected %v %v", tag, err)
	}
	agErr := testApplyGetterAll{initErr: ErrNotComparable}
	if _, err := elem.ApplyGetter(agErr); err != ErrNotComparable {
		t.Fatalf("expected init err, got %v", err)
	}

	// This branch only exists in deadlock debug builds.
	if deadlock.Debug {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for non-comparable UI in debug mode")
			}
		}()
		rq.NewElement(testUnhashableUI{m: map[string]int{"x": 1}})
	}
}

type testUnhashableUI struct {
	m map[string]int
}

func (testUnhashableUI) JawsRender(*Element, io.Writer, []any) error { return nil }
func (testUnhashableUI) JawsUpdate(*Element)                         {}
