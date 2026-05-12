package ui

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/named"
	"github.com/linkdata/jaws/lib/what"
)

func TestContainerAndTbodyRender(t *testing.T) {
	_, rq := newCoreRequest(t)
	tc := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("foo")), NewSpan(testHTMLGetter("bar"))}}

	container := NewContainer("div", tc)
	_, got := renderUI(t, rq, container, "hidden")
	mustMatch(t, `^<div id="Jid\.[0-9]+" hidden><span id="Jid\.[0-9]+">foo</span><span id="Jid\.[0-9]+">bar</span></div>$`, got)

	tbody := NewTbody(tc)
	elem, got := renderUI(t, rq, tbody)
	mustMatch(t, `^<tbody id="Jid\.[0-9]+"><span id="Jid\.[0-9]+">foo</span><span id="Jid\.[0-9]+">bar</span></tbody>$`, got)
	tbody.JawsUpdate(elem)
}

func TestContainerHelperUpdateContainer(t *testing.T) {
	_, rq := newCoreRequest(t)
	span1 := NewSpan(testHTMLGetter("span1"))
	span2 := NewSpan(testHTMLGetter("span2"))
	span3 := NewSpan(testHTMLGetter("span3"))

	tc := &testContainer{contents: []jaws.UI{span1}}
	container := NewContainer("div", tc)
	elem, _ := renderUI(t, rq, container)

	if len(container.contents) != 1 {
		t.Fatalf("want 1 content got %d", len(container.contents))
	}

	// append + reorder path
	tc.contents = []jaws.UI{span1, span2, span3}
	container.JawsUpdate(elem)
	if len(container.contents) != 3 {
		t.Fatalf("want 3 contents got %d", len(container.contents))
	}

	// remove path
	removedJid := container.contents[0].Jid()
	tc.contents = []jaws.UI{span2, span3}
	container.JawsUpdate(elem)
	if got := rq.GetElementByJid(removedJid); got != nil {
		t.Fatal("expected removed element to be deleted from request")
	}

	// reorder + replace path
	tc.contents = []jaws.UI{span3, span1}
	container.JawsUpdate(elem)
	if len(container.contents) != 2 {
		t.Fatalf("want 2 contents got %d", len(container.contents))
	}
}

func TestContainerHelperUpdateContainerDuplicates(t *testing.T) {
	_, rq := newCoreRequest(t)
	span1 := NewSpan(testHTMLGetter("span1"))
	span2 := NewSpan(testHTMLGetter("span2"))

	// render with duplicate UI
	tc := &testContainer{contents: []jaws.UI{span1, span2, span1}}
	container := NewContainer("div", tc)
	elem, _ := renderUI(t, rq, container)

	if len(container.contents) != 3 {
		t.Fatalf("want 3 contents got %d", len(container.contents))
	}
	// the two span1 Elements must have distinct Jids
	jid0 := container.contents[0].Jid()
	jid2 := container.contents[2].Jid()
	if jid0 == jid2 {
		t.Fatal("duplicate UI must produce distinct Jids")
	}

	// remove one duplicate, keep the other
	tc.contents = []jaws.UI{span2, span1}
	container.JawsUpdate(elem)
	if len(container.contents) != 2 {
		t.Fatalf("want 2 contents got %d", len(container.contents))
	}
	// one of the two span1 Jids should have been removed
	kept := container.contents[1].Jid()
	if kept != jid0 && kept != jid2 {
		t.Fatalf("expected kept Jid to be one of the original span1 Jids")
	}
	var removedJid jid.Jid
	if kept == jid0 {
		removedJid = jid2
	} else {
		removedJid = jid0
	}
	if got := rq.GetElementByJid(removedJid); got != nil {
		t.Fatal("expected surplus duplicate to be deleted from request")
	}

	// add more duplicates
	tc.contents = []jaws.UI{span1, span2, span1, span2}
	container.JawsUpdate(elem)
	if len(container.contents) != 4 {
		t.Fatalf("want 4 contents got %d", len(container.contents))
	}
	// all four must have distinct Jids
	jids := make(map[jid.Jid]struct{}, 4)
	for i, c := range container.contents {
		if _, ok := jids[c.Jid()]; ok {
			t.Fatalf("contents[%d] has duplicate Jid %v", i, c.Jid())
		}
		jids[c.Jid()] = struct{}{}
	}
}

func TestContainerHelperRenderErrorPaths(t *testing.T) {
	jaws.NextJid = 0
	_, rq := newCoreRequest(t)
	renderErr := errors.New("render error")
	errChild := testRenderErrorUI{err: renderErr}
	tc := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("first")), errChild, NewSpan(testHTMLGetter("third"))}}

	container := NewContainer("div", tc)
	elem := rq.NewElement(container)
	var sb strings.Builder
	err := elem.JawsRender(&sb, nil)
	if !errors.Is(err, renderErr) {
		t.Fatalf("want %v got %v", renderErr, err)
	}
	if len(container.contents) != 0 {
		t.Fatalf("want 0 successful child got %d", len(container.contents))
	}

	// panic path from must() during append
	tc2 := &testContainer{}
	container2 := NewContainer("div", tc2)
	elem2, _ := renderUI(t, rq, container2)
	tc2.contents = []jaws.UI{testRenderErrorUI{err: errors.New("append fail")}}
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic from must")
		}
	}()
	container2.JawsUpdate(elem2)
}

type testRenderErrorUI struct {
	err error
}

func (u testRenderErrorUI) JawsRender(*jaws.Element, io.Writer, []any) error {
	return u.err
}

func (testRenderErrorUI) JawsUpdate(*jaws.Element) {}

type testRenderErrorCaptureUI struct {
	err error
	jid jaws.Jid
}

func (u *testRenderErrorCaptureUI) JawsRender(e *jaws.Element, _ io.Writer, _ []any) error {
	u.jid = e.Jid()
	return u.err
}

func (*testRenderErrorCaptureUI) JawsUpdate(*jaws.Element) {}

func TestContainerHelperRenderErrorDoesNotLeakFailedChildElement(t *testing.T) {
	jaws.NextJid = 0
	_, rq := newCoreRequest(t)

	renderErr := errors.New("render error")
	failingChild := &testRenderErrorCaptureUI{err: renderErr}
	tc := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("ok")), failingChild}}
	container := NewContainer("div", tc)

	elem := rq.NewElement(container)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); !errors.Is(err, renderErr) {
		t.Fatalf("want %v got %v", renderErr, err)
	}

	if !failingChild.jid.IsValid() {
		t.Fatal("expected failing child jid to be captured")
	}
	if leaked := rq.GetElementByJid(failingChild.jid); leaked != nil {
		t.Fatalf("expected failed child %v to be removed from request registry", failingChild.jid)
	}
}

func TestRequestWriterUI_ContainerRenderErrorDoesNotLeakSuccessfulChildren(t *testing.T) {
	jaws.NextJid = 0
	_, rq := newCoreRequest(t)
	var sb strings.Builder
	rw := RequestWriter{Request: rq, Writer: &sb}

	renderErr := errors.New("render error")
	okChild := &testRenderErrorCaptureUI{}
	failChild := &testRenderErrorCaptureUI{err: renderErr}
	tc := &testContainer{contents: []jaws.UI{okChild, failChild}}

	if err := rw.UI(NewContainer("div", tc)); !errors.Is(err, renderErr) {
		t.Fatalf("want %v got %v", renderErr, err)
	}

	if !okChild.jid.IsValid() {
		t.Fatal("expected successful child jid to be captured")
	}
	if leaked := rq.GetElementByJid(okChild.jid); leaked != nil {
		t.Fatalf("expected successful child %v to be removed when parent render fails", okChild.jid)
	}
}

type testSelectHandler struct {
	*testContainer
	*testSetter[string]
}

func TestSelectWidget(t *testing.T) {
	_, rq := newCoreRequest(t)
	sh := &testSelectHandler{
		testContainer: &testContainer{contents: []jaws.UI{NewOption(named.NewBool(nil, "1", "one", true))}},
		testSetter:    newTestSetter("1"),
	}
	selectUI := NewSelect(sh)
	elem, got := renderUI(t, rq, selectUI)
	mustMatch(t, `^<select id="Jid\.[0-9]+"><option id="Jid\.[0-9]+" value="1" selected>one</option></select>$`, got)

	selectUI.JawsUpdate(elem)

	if err := jaws.CallEventHandlers(selectUI, elem, what.Click, "1 2 0 noop"); !errors.Is(err, jaws.ErrEventUnhandled) {
		t.Fatalf("want ErrEventUnhandled got %v", err)
	}
	if err := selectUI.JawsInput(elem, "2"); err != nil {
		t.Fatal(err)
	}
	if sh.Get() != "2" {
		t.Fatalf("want 2 got %q", sh.Get())
	}
	sh.SetErr(errors.New("meh"))
	if err := selectUI.JawsInput(elem, "3"); err == nil || err.Error() != "meh" {
		t.Fatalf("want meh got %v", err)
	}
}
