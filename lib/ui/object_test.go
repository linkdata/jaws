package ui

import (
	"errors"
	"fmt"
	"html/template"
	"strings"
	"sync"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
)

type testObjectStringer struct {
	s string
}

func (s testObjectStringer) String() string {
	return s.s
}

type testObjectTagGetter struct {
	v any
}

func (g testObjectTagGetter) JawsGetTag() any {
	return g.v
}

type testObjectHTMLGetter struct{}

func (*testObjectHTMLGetter) JawsGetHTML(*jaws.Element) template.HTML { return "text" }

type testObjectBinderNoHTML struct{ bind.Binder[string] }

func TestObject_NewForwardsHTMLAndTag(t *testing.T) {
	_, rq := newCoreRequest(t)
	elem := rq.NewElement(NewSpan(testHTMLGetter("x")))
	inner := testObjectStringer{s: "<b>x</b>"}
	obj := New(inner)

	if got, want := string(obj.JawsGetHTML(elem)), "&lt;b&gt;x&lt;/b&gt;"; got != want {
		t.Fatalf("want %q got %q", want, got)
	}
	if got, want := obj.JawsGetTag(), any(inner); got != want {
		t.Fatalf("want tag %#v got %#v", want, got)
	}
}

func TestObject_InnerHTMLGetterWithoutTag(t *testing.T) {
	_, rq := newCoreRequest(t)
	inner := new(testObjectHTMLGetter)
	direct, _ := renderUI(t, rq, NewSpan(inner))
	if tags := rq.TagsOf(direct); len(tags) != 1 || tags[0] != inner {
		t.Fatalf("direct HTMLGetter tags = %#v, want inner value", tags)
	}
	wrapped, _ := renderUI(t, rq, NewSpan(New(inner)))
	if tags := rq.TagsOf(wrapped); len(tags) != 0 {
		t.Fatalf("wrapped HTMLGetter tags = %#v, want none", tags)
	}
}

func TestObject_InnerBinderHooks(t *testing.T) {
	_, rq := newCoreRequest(t)
	var mu sync.Mutex
	value := "text"
	clicks, menus, attrs := 0, 0, 0
	binder := bind.New(&mu, &value).
		Clicked(func(bind.Binder[string], *jaws.Element, jaws.Click) error {
			clicks++
			return nil
		}).
		ContextMenu(func(bind.Binder[string], *jaws.Element, jaws.Click) error {
			menus++
			return nil
		}).
		InitialHTMLAttr(func(bind.Binder[string], *jaws.Element) template.HTMLAttr {
			attrs++
			return `data-inner="yes"`
		})

	direct, directHTML := renderUI(t, rq, NewSpan(binder))
	if !strings.Contains(directHTML, `data-inner="yes"`) {
		t.Fatalf("direct Binder HTML = %q, want initial attribute", directHTML)
	}
	if err := jaws.CallEventHandlers(direct.UI(), direct, what.Click, "1 2 0 click"); err != nil {
		t.Fatal(err)
	}
	if err := jaws.CallEventHandlers(direct.UI(), direct, what.ContextMenu, "1 2 0 menu"); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 || menus != 1 || attrs != 1 {
		t.Fatalf("direct Binder calls = (%d, %d, %d), want (1, 1, 1)", clicks, menus, attrs)
	}
	adapted, adaptedHTML := renderUI(t, rq, NewSpan(testObjectBinderNoHTML{binder}))
	if !strings.Contains(adaptedHTML, `data-inner="yes"`) {
		t.Fatalf("adapted Binder HTML = %q, want initial attribute", adaptedHTML)
	}
	if err := jaws.CallEventHandlers(adapted.UI(), adapted, what.Click, "1 2 0 click"); err != nil {
		t.Fatal(err)
	}
	if err := jaws.CallEventHandlers(adapted.UI(), adapted, what.ContextMenu, "1 2 0 menu"); err != nil {
		t.Fatal(err)
	}
	if clicks != 2 || menus != 2 || attrs != 2 {
		t.Fatalf("adapted Binder calls = (%d, %d, %d), want (2, 2, 2)", clicks, menus, attrs)
	}

	wrapped, wrappedHTML := renderUI(t, rq, NewSpan(New(binder)))
	if strings.Contains(wrappedHTML, `data-inner="yes"`) || !strings.Contains(wrappedHTML, ">text</span>") {
		t.Fatalf("wrapped Binder HTML = %q, want content without inner attribute", wrappedHTML)
	}
	if tags := rq.TagsOf(wrapped); len(tags) != 1 || tags[0] != &value {
		t.Fatalf("wrapped Binder tags = %#v, want backing pointer", tags)
	}
	if err := jaws.CallEventHandlers(wrapped.UI(), wrapped, what.Click, "1 2 0 click"); !errors.Is(err, jaws.ErrEventUnhandled) {
		t.Fatalf("wrapped click = %v, want ErrEventUnhandled", err)
	}
	if err := jaws.CallEventHandlers(wrapped.UI(), wrapped, what.ContextMenu, "1 2 0 menu"); !errors.Is(err, jaws.ErrEventUnhandled) {
		t.Fatalf("wrapped context menu = %v, want ErrEventUnhandled", err)
	}
	if clicks != 2 || menus != 2 || attrs != 2 {
		t.Fatalf("inner Binder calls after wrapping = (%d, %d, %d), want (2, 2, 2)", clicks, menus, attrs)
	}
}

func TestObject_GetTag_SurvivesChaining(t *testing.T) {
	inner := testObjectStringer{s: "x"}
	obj := New(inner).
		Clicked(func(Object, *jaws.Element, jaws.Click) error { return nil }).
		ContextMenu(func(Object, *jaws.Element, jaws.Click) error { return nil }).
		InitialHTMLAttr(func(Object, *jaws.Element) template.HTMLAttr { return "" })

	got, err := tag.TagExpand(obj)
	if err != nil {
		t.Fatalf("TagExpand() error = %v", err)
	}
	if len(got) != 1 || got[0] != inner {
		t.Fatalf("TagExpand() = %#v, want [%#v]", got, inner)
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
	order := []int{}
	gotObj := []Object{}
	gotElem := []*jaws.Element{}
	gotClick := []jaws.Click{}

	first := New("x").Clicked(func(got Object, elem *jaws.Element, click jaws.Click) error {
		order = append(order, 1)
		gotObj = append(gotObj, got)
		gotElem = append(gotElem, elem)
		gotClick = append(gotClick, click)
		return nil
	})
	obj := first.Clicked(func(got Object, elem *jaws.Element, click jaws.Click) error {
		order = append(order, 2)
		gotObj = append(gotObj, got)
		gotElem = append(gotElem, elem)
		gotClick = append(gotClick, click)
		return jaws.ErrEventUnhandled
	}).InitialHTMLAttr(func(Object, *jaws.Element) template.HTMLAttr { return "" })

	elem := &jaws.Element{}
	click := jaws.Click{Name: "save", X: 1, Y: 2}
	if err := obj.JawsClick(elem, click); err != nil {
		t.Fatalf("want nil got %v", err)
	}
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Fatalf("unexpected order %v", order)
	}
	if gotObj[0] != obj || gotObj[1] != obj {
		t.Fatalf("want invoking object %p for both hooks, got [%p %p]", obj, gotObj[0], gotObj[1])
	}
	if gotElem[0] != elem || gotElem[1] != elem {
		t.Fatalf("unexpected elem forwarding %#v", gotElem)
	}
	if gotClick[0] != click || gotClick[1] != click {
		t.Fatalf("unexpected click forwarding %#v", gotClick)
	}
	order, gotObj = nil, nil
	if err := first.JawsClick(elem, click); err != nil {
		t.Fatalf("intermediate JawsClick() = %v", err)
	}
	if len(order) != 1 || order[0] != 1 || len(gotObj) != 1 || gotObj[0] != first {
		t.Fatalf("intermediate call: order %v, objects %v; want [1], [%p]", order, gotObj, first)
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
	if called1 != 0 {
		t.Fatalf("want first not called, got %d", called1)
	}
	if called2 != 1 {
		t.Fatalf("want second called once, got %d", called2)
	}
}

func TestObject_ContextMenu_FallthroughOrder(t *testing.T) {
	order := []int{}
	gotObj := []Object{}

	first := New("x").ContextMenu(func(got Object, _ *jaws.Element, _ jaws.Click) error {
		order = append(order, 1)
		gotObj = append(gotObj, got)
		return nil
	})
	obj := first.ContextMenu(func(got Object, _ *jaws.Element, _ jaws.Click) error {
		order = append(order, 2)
		gotObj = append(gotObj, got)
		return jaws.ErrEventUnhandled
	}).Clicked(func(Object, *jaws.Element, jaws.Click) error { return nil })

	if err := obj.JawsContextMenu(nil, jaws.Click{Name: "menu"}); err != nil {
		t.Fatalf("want nil got %v", err)
	}
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Fatalf("unexpected order %v", order)
	}
	if gotObj[0] != obj || gotObj[1] != obj {
		t.Fatalf("want invoking object %p for both hooks, got [%p %p]", obj, gotObj[0], gotObj[1])
	}
	order, gotObj = nil, nil
	if err := first.JawsContextMenu(nil, jaws.Click{Name: "menu"}); err != nil {
		t.Fatalf("intermediate JawsContextMenu() = %v", err)
	}
	if len(order) != 1 || order[0] != 1 || len(gotObj) != 1 || gotObj[0] != first {
		t.Fatalf("intermediate call: order %v, objects %v; want [1], [%p]", order, gotObj, first)
	}
}

func TestObject_EventUnhandledCanBeWrapped(t *testing.T) {
	older := fmt.Errorf("older: %w", jaws.ErrEventUnhandled)
	newer := fmt.Errorf("newer: %w", jaws.ErrEventUnhandled)
	clicked := []int{}
	obj := New("x").
		Clicked(func(Object, *jaws.Element, jaws.Click) error {
			clicked = append(clicked, 1)
			return older
		}).
		Clicked(func(Object, *jaws.Element, jaws.Click) error {
			clicked = append(clicked, 2)
			return newer
		})
	if err := obj.JawsClick(nil, jaws.Click{Name: "click"}); !errors.Is(err, jaws.ErrEventUnhandled) {
		t.Fatalf("want ErrEventUnhandled chain got %v", err)
	}
	if len(clicked) != 2 || clicked[0] != 2 || clicked[1] != 1 {
		t.Fatalf("unexpected wrapped click fallthrough order %v", clicked)
	}

	contextMenu := []int{}
	obj = New("x").
		ContextMenu(func(Object, *jaws.Element, jaws.Click) error {
			contextMenu = append(contextMenu, 1)
			return older
		}).
		ContextMenu(func(Object, *jaws.Element, jaws.Click) error {
			contextMenu = append(contextMenu, 2)
			return newer
		})
	if err := obj.JawsContextMenu(nil, jaws.Click{Name: "menu"}); !errors.Is(err, jaws.ErrEventUnhandled) {
		t.Fatalf("want ErrEventUnhandled chain got %v", err)
	}
	if len(contextMenu) != 2 || contextMenu[0] != 2 || contextMenu[1] != 1 {
		t.Fatalf("unexpected wrapped context-menu fallthrough order %v", contextMenu)
	}
}

func TestObject_InitialHTMLAttr_DefaultEmpty(t *testing.T) {
	obj := New("x")
	if got := obj.JawsInitialHTMLAttr(nil); got != "" {
		t.Fatalf("want empty attr got %q", got)
	}
}

func TestObject_InitialHTMLAttr(t *testing.T) {
	type initialHTMLAttrHook func(Object, *jaws.Element) template.HTMLAttr

	order := []int{}
	gotObj := []Object{}
	elem := &jaws.Element{}
	first := initialHTMLAttrHook(func(got Object, gotElem *jaws.Element) (s template.HTMLAttr) {
		order = append(order, 1)
		gotObj = append(gotObj, got)
		if gotElem != elem {
			t.Fatalf("unexpected elem %#v", gotElem)
		}
		return `data-first="1"`
	})

	firstObj := New("x").InitialHTMLAttr(first)
	obj := firstObj.InitialHTMLAttr(func(got Object, _ *jaws.Element) (s template.HTMLAttr) {
		order = append(order, 2)
		gotObj = append(gotObj, got)
		s = `data-second="2"`
		return
	}).Clicked(func(Object, *jaws.Element, jaws.Click) error { return nil })

	if got := obj.JawsInitialHTMLAttr(elem); got != `data-second="2" data-first="1"` {
		t.Fatalf("want %q got %q", `data-second="2" data-first="1"`, got)
	}
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Fatalf("unexpected order %v", order)
	}
	if gotObj[0] != obj || gotObj[1] != obj {
		t.Fatalf("want invoking object %p for both hooks, got [%p %p]", obj, gotObj[0], gotObj[1])
	}
	order, gotObj = nil, nil
	if got := firstObj.JawsInitialHTMLAttr(elem); got != `data-first="1"` {
		t.Fatalf("intermediate JawsInitialHTMLAttr() = %q", got)
	}
	if len(order) != 1 || order[0] != 1 || len(gotObj) != 1 || gotObj[0] != firstObj {
		t.Fatalf("intermediate call: order %v, objects %v; want [1], [%p]", order, gotObj, firstObj)
	}
}

func TestObject_GetTag_MultipleTagsReturnsSlice(t *testing.T) {
	obj := &object{
		handler: testObjectTagGetter{v: "top"},
		prev: &object{
			handler: testObjectTagGetter{v: "prev"},
		},
	}

	got := obj.JawsGetTag()
	tags, ok := got.([]any)
	if !ok {
		t.Fatalf("want []any got %T (%#v)", got, got)
	}
	if len(tags) != 2 {
		t.Fatalf("want 2 tags got %d (%#v)", len(tags), tags)
	}
	if tags[0] != "top" || tags[1] != "prev" {
		t.Fatalf("want [top prev] got %#v", tags)
	}
}
