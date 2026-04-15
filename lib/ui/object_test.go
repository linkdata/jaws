package ui

import (
	"errors"
	"fmt"
	"html/template"
	"testing"

	"github.com/linkdata/jaws"
)

type testObjectStringer struct {
	s string
}

func (s testObjectStringer) String() string {
	return s.s
}

func TestObject_NewForwardsHTMLAndTag(t *testing.T) {
	_, rq := newCoreRequest(t)
	elem := rq.NewElement(NewSpan(testHTMLGetter("x")))
	inner := testObjectStringer{s: "<b>x</b>"}
	obj := New(inner)

	if got, want := string(obj.JawsGetHTML(elem)), "&lt;b&gt;x&lt;/b&gt;"; got != want {
		t.Fatalf("want %q got %q", want, got)
	}
	if got, want := obj.JawsGetTag(rq), any(inner); got != want {
		t.Fatalf("want tag %#v got %#v", want, got)
	}
}

func TestObject_Click_DefaultUnhandled(t *testing.T) {
	obj := New("x")
	if err := obj.JawsClick(nil, jaws.Click{Name: "ignored"}); err != jaws.ErrEventUnhandled {
		t.Fatalf("want ErrEventUnhandled got %v", err)
	}
}

func TestObject_ContextMenu_DefaultUnhandled(t *testing.T) {
	obj := New("x")
	if err := obj.JawsContextMenu(nil, jaws.Click{Name: "ignored"}); err != jaws.ErrEventUnhandled {
		t.Fatalf("want ErrEventUnhandled got %v", err)
	}
}

func TestObject_Clicked_FallthroughOrder(t *testing.T) {
	obj := New("x")
	order := []int{}
	gotObj := []Object{}
	gotElem := []*jaws.Element{}
	gotClick := []jaws.Click{}

	obj = obj.Clicked(func(got Object, elem *jaws.Element, click jaws.Click) error {
		order = append(order, 1)
		gotObj = append(gotObj, got)
		gotElem = append(gotElem, elem)
		gotClick = append(gotClick, click)
		return jaws.ErrEventUnhandled
	}).Clicked(func(got Object, elem *jaws.Element, click jaws.Click) error {
		order = append(order, 2)
		gotObj = append(gotObj, got)
		gotElem = append(gotElem, elem)
		gotClick = append(gotClick, click)
		return nil
	})

	elem := &jaws.Element{}
	click := jaws.Click{Name: "save", X: 1, Y: 2}
	if err := obj.JawsClick(elem, click); err != nil {
		t.Fatalf("want nil got %v", err)
	}
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("unexpected order %v", order)
	}
	if gotObj[0] == gotObj[1] {
		t.Fatalf("expected distinct hook objects")
	}
	if gotElem[0] != elem || gotElem[1] != elem {
		t.Fatalf("unexpected elem forwarding %#v", gotElem)
	}
	if gotClick[0] != click || gotClick[1] != click {
		t.Fatalf("unexpected click forwarding %#v", gotClick)
	}
}

func TestObject_Clicked_StopsOnHandled(t *testing.T) {
	called1 := 0
	called2 := 0
	obj := New("x").
		Clicked(func(Object, *jaws.Element, jaws.Click) error {
			called1++
			return nil
		}).
		Clicked(func(Object, *jaws.Element, jaws.Click) error {
			called2++
			return nil
		})

	if err := obj.JawsClick(nil, jaws.Click{Name: "save"}); err != nil {
		t.Fatalf("want nil got %v", err)
	}
	if called1 != 1 {
		t.Fatalf("want first called once, got %d", called1)
	}
	if called2 != 0 {
		t.Fatalf("want second not called, got %d", called2)
	}
}

func TestObject_ContextMenu_FallthroughOrder(t *testing.T) {
	obj := New("x")
	order := []int{}

	obj = obj.ContextMenu(func(Object, *jaws.Element, jaws.Click) error {
		order = append(order, 1)
		return jaws.ErrEventUnhandled
	}).ContextMenu(func(Object, *jaws.Element, jaws.Click) error {
		order = append(order, 2)
		return nil
	})

	if err := obj.JawsContextMenu(nil, jaws.Click{Name: "menu"}); err != nil {
		t.Fatalf("want nil got %v", err)
	}
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("unexpected order %v", order)
	}
}

func TestObject_EventUnhandledCanBeWrapped(t *testing.T) {
	wrapped := fmt.Errorf("wrapped: %w", jaws.ErrEventUnhandled)

	obj := New("x").Clicked(func(Object, *jaws.Element, jaws.Click) error {
		return wrapped
	})
	if err := obj.JawsClick(nil, jaws.Click{Name: "click"}); !errors.Is(err, jaws.ErrEventUnhandled) {
		t.Fatalf("want ErrEventUnhandled chain got %v", err)
	} else if err != wrapped {
		t.Fatalf("want wrapped unhandled got %v", err)
	}

	obj = New("x").ContextMenu(func(Object, *jaws.Element, jaws.Click) error {
		return wrapped
	})
	if err := obj.JawsContextMenu(nil, jaws.Click{Name: "menu"}); !errors.Is(err, jaws.ErrEventUnhandled) {
		t.Fatalf("want ErrEventUnhandled chain got %v", err)
	} else if err != wrapped {
		t.Fatalf("want wrapped unhandled got %v", err)
	}
}

func TestObject_InitialHTMLAttr_DefaultEmpty(t *testing.T) {
	obj := New("x")
	if got := obj.JawsInitialHTMLAttr(nil); got != "" {
		t.Fatalf("want empty attr got %q", got)
	}
}

func TestObject_InitialHTMLAttr_FirstHookWins(t *testing.T) {
	order := []int{}
	elem := &jaws.Element{}

	obj := New("x").
		InitialHTMLAttr(func(got Object, gotElem *jaws.Element) (s template.HTMLAttr) {
			order = append(order, 1)
			if gotElem != elem {
				t.Fatalf("unexpected elem %#v", gotElem)
			}
			return `data-first="1"`
		}).
		InitialHTMLAttr(func(Object, *jaws.Element) (s template.HTMLAttr) {
			order = append(order, 2)
			s = `data-second="2"`
			return
		})

	if got := obj.JawsInitialHTMLAttr(elem); got != `data-first="1"` {
		t.Fatalf("want %q got %q", `data-first="1"`, got)
	}
	if len(order) != 1 || order[0] != 1 {
		t.Fatalf("unexpected order %v", order)
	}
}
