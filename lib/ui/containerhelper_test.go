package ui

import (
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/named"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
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

// TestContainerHelper_UpdateEmitsWireOps pins the browser-visible wire output of
// UpdateContainer: appending a child must emit an Append carrying that child's
// rendered HTML and an Order reflecting the new sequence. Asserting the ops (not
// just the in-memory contents slice) catches regressions that line coverage misses.
func TestContainerHelper_UpdateEmitsWireOps(t *testing.T) {
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

	span1 := NewSpan(testHTMLGetter("span1"))
	span2 := NewSpan(testHTMLGetter("span2"))
	tc := &testContainer{contents: []jaws.UI{span1}}
	container := NewContainer("div", tc)
	elem := tr.NewElement(container)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}

	tc.contents = []jaws.UI{span1, span2}
	container.JawsUpdate(elem)
	// Wake the harness loop so the queued ops flush to OutCh.
	tr.InCh <- wire.WsMsg{}

	var sawAppend, sawOrder bool
collect:
	for {
		select {
		case msg := <-tr.OutCh:
			switch msg.What {
			case what.Append:
				sawAppend = true
				if !strings.Contains(msg.Data, "span2") {
					t.Errorf("Append data %q does not contain the new child's HTML", msg.Data)
				}
			case what.Order:
				sawOrder = true
			}
		case <-time.After(300 * time.Millisecond):
			break collect
		}
	}
	if !sawAppend || !sawOrder {
		t.Fatalf("want both Append and Order ops, got append=%v order=%v", sawAppend, sawOrder)
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

// TestContainerHelper_ReconcileDiscardsOutOfBandDeletedChild verifies that a child
// Element deleted out-of-band (e.g. by a what.Delete broadcast on a shared tag, or a
// browser what.Remove) is not reused from the reconcile pool. A deleted Element is
// inert, so reusing it would leave the still-wanted child permanently unrendered and
// put a phantom Jid in the Order; reconcile must create a fresh Element instead.
func TestContainerHelper_ReconcileDiscardsOutOfBandDeletedChild(t *testing.T) {
	_, rq := newCoreRequest(t)
	span1 := NewSpan(testHTMLGetter("span1"))
	span2 := NewSpan(testHTMLGetter("span2"))
	tc := &testContainer{contents: []jaws.UI{span1, span2}}
	container := NewContainer("div", tc)
	elem, _ := renderUI(t, rq, container)

	if len(container.contents) != 2 {
		t.Fatalf("want 2 contents got %d", len(container.contents))
	}
	deletedChild := container.contents[1]
	deletedJid := deletedChild.Jid()

	// Delete span2's Element out-of-band while the container still wants it.
	rq.DeleteElement(deletedChild)
	if !deletedChild.Deleted() {
		t.Fatal("expected child to be marked deleted")
	}

	// tc.contents is unchanged (still wants span1 and span2), so reconcile must not
	// reuse the deleted Element for span2.
	container.JawsUpdate(elem)

	if len(container.contents) != 2 {
		t.Fatalf("want 2 contents after update got %d", len(container.contents))
	}
	fresh := container.contents[1]
	if fresh == deletedChild || fresh.Jid() == deletedJid {
		t.Fatal("reconcile reused the deleted Element instead of creating a fresh one")
	}
	if fresh.Deleted() {
		t.Fatal("replacement child must not be deleted")
	}
	if rq.GetElementByJid(fresh.Jid()) == nil {
		t.Fatal("replacement child must be registered in the request")
	}
}

func TestContainerHelper_RemovesOutOfBandDeletedLeftover(t *testing.T) {
	_, rq := newCoreRequest(t) // jaws.New(): nil Logger, so a spurious reportMisuse panics
	span1 := NewSpan(testHTMLGetter("span1"))
	span2 := NewSpan(testHTMLGetter("span2"))
	tc := &testContainer{contents: []jaws.UI{span1, span2}}
	container := NewContainer("div", tc)
	elem, _ := renderUI(t, rq, container)

	if len(container.contents) != 2 {
		t.Fatalf("want 2 contents got %d", len(container.contents))
	}
	deletedChild := container.contents[1]

	// Delete span2's Element out-of-band (as a what.Delete broadcast would via
	// rq.DeleteElement) AND drop span2 from what the container wants. That routes the
	// deleted Element to the removal path rather than the self-healing reuse path
	// exercised by TestContainerHelper_ReconcileDiscardsOutOfBandDeletedChild. Before
	// the fix, UpdateContainer called elem.Remove on the deleted leftover and
	// validChildElement reported it via reportMisuse, panicking with the nil Logger.
	rq.DeleteElement(deletedChild)
	if !deletedChild.Deleted() {
		t.Fatal("expected child to be marked deleted")
	}
	tc.contents = []jaws.UI{span1}

	container.JawsUpdate(elem)

	if len(container.contents) != 1 {
		t.Fatalf("want 1 content after update got %d", len(container.contents))
	}
	if remaining := container.contents[0]; remaining.Deleted() || rq.GetElementByJid(remaining.Jid()) == nil {
		t.Fatal("remaining child must be live and registered")
	}
}

func TestContainerHelperRenderErrorPaths(t *testing.T) {
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

func TestContainerHelperUpdateRenderErrorDoesNotAppendFailedChild(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	jw.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	defer tr.Close()
	<-tr.ReadyCh

	tc := &testContainer{}
	container := NewContainer("div", tc)
	elem := tr.NewElement(container)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}

	renderErr := errors.New("append render failed")
	failingChild := &testRenderErrorCaptureUI{err: renderErr}
	tc.contents = []jaws.UI{failingChild}
	container.JawsUpdate(elem)

	if !failingChild.jid.IsValid() {
		t.Fatal("expected failing child jid to be captured")
	}
	if leaked := tr.GetElementByJid(failingChild.jid); leaked != nil {
		t.Fatalf("failed append child %v leaked into the request registry", failingChild.jid)
	}

	tr.InCh <- wire.WsMsg{}
	select {
	case msg := <-tr.OutCh:
		if msg.What == what.Append || msg.What == what.Order {
			t.Fatalf("failed append render emitted browser mutation: %+v", msg)
		}
	case <-time.After(300 * time.Millisecond):
	}
}

type testRenderErrorUI struct {
	err error
}

func (u testRenderErrorUI) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.err
}

func (testRenderErrorUI) JawsUpdate(elem *jaws.Element) {}

type testRenderErrorCaptureUI struct {
	err error
	jid jaws.Jid
}

func (u *testRenderErrorCaptureUI) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	u.jid = elem.Jid()
	return u.err
}

func (*testRenderErrorCaptureUI) JawsUpdate(elem *jaws.Element) {}

type failNthWrite struct {
	n   int
	err error
}

func (w *failNthWrite) Write(p []byte) (int, error) {
	w.n--
	if w.n == 0 {
		return 0, w.err
	}
	return len(p), nil
}

func TestRequestWriterUI_ContainerClosingWriteErrorDoesNotLeakChildren(t *testing.T) {
	_, rq := newCoreRequest(t)

	writeErr := errors.New("closing write failed")
	child := &testRenderErrorCaptureUI{}
	tc := &testContainer{contents: []jaws.UI{child}}
	writer := &failNthWrite{n: 2, err: writeErr}
	rw := RequestWriter{Request: rq, Writer: writer}

	if err := rw.NewUI(NewContainer("div", tc)); !errors.Is(err, writeErr) {
		t.Fatalf("want %v got %v", writeErr, err)
	}

	if !child.jid.IsValid() {
		t.Fatal("expected child jid to be captured")
	}
	if leaked := rq.GetElementByJid(child.jid); leaked != nil {
		t.Fatalf("expected child %v to be removed when parent closing write fails", child.jid)
	}
}

type benchContainer struct {
	contents []jaws.UI
}

func (bc *benchContainer) JawsContains(elem *jaws.Element) []jaws.UI {
	return bc.contents
}

type benchChild struct {
	id int
}

func (child benchChild) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	_, err := io.WriteString(w, `<span>child</span>`)
	return err
}

func (benchChild) JawsUpdate(elem *jaws.Element) {}

func benchRequest(b *testing.B) (*jaws.Jaws, *jaws.Request) {
	b.Helper()
	jw, err := jaws.New()
	if err != nil {
		b.Fatal(err)
	}
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		jw.Close()
		b.Fatal("nil request")
	}
	return jw, rq
}

func benchChildren(start, count int) []jaws.UI {
	contents := make([]jaws.UI, count)
	for i := range contents {
		contents[i] = benchChild{id: start + i}
	}
	return contents
}

func BenchmarkContainerHelperUpdateAppendHeavy(b *testing.B) {
	b.ReportAllocs()
	const size = 1000
	for range b.N {
		b.StopTimer()
		jw, rq := benchRequest(b)
		bc := &benchContainer{}
		container := NewContainer("div", bc)
		elem := rq.NewElement(container)
		if err := elem.JawsRender(io.Discard, nil); err != nil {
			b.Fatal(err)
		}
		bc.contents = benchChildren(0, size)
		b.StartTimer()
		container.JawsUpdate(elem)
		b.StopTimer()
		jw.Close()
	}
}

func BenchmarkContainerHelperUpdateMixed(b *testing.B) {
	b.ReportAllocs()
	const size = 1000
	for range b.N {
		b.StopTimer()
		jw, rq := benchRequest(b)
		bc := &benchContainer{contents: benchChildren(0, size)}
		container := NewContainer("div", bc)
		elem := rq.NewElement(container)
		if err := elem.JawsRender(io.Discard, nil); err != nil {
			b.Fatal(err)
		}
		next := make([]jaws.UI, 0, size)
		for i := size / 2; i < size; i++ {
			next = append(next, benchChild{id: i})
		}
		for i := size; i < size+size/2; i++ {
			next = append(next, benchChild{id: i})
		}
		bc.contents = next
		b.StartTimer()
		container.JawsUpdate(elem)
		b.StopTimer()
		jw.Close()
	}
}

func TestContainerHelperRenderErrorDoesNotLeakFailedChildElement(t *testing.T) {
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
	_, rq := newCoreRequest(t)
	var sb strings.Builder
	rw := RequestWriter{Request: rq, Writer: &sb}

	renderErr := errors.New("render error")
	okChild := &testRenderErrorCaptureUI{}
	failChild := &testRenderErrorCaptureUI{err: renderErr}
	tc := &testContainer{contents: []jaws.UI{okChild, failChild}}

	if err := rw.NewUI(NewContainer("div", tc)); !errors.Is(err, renderErr) {
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

type plainSelectOption struct {
	value string
	label string
}

func (opt plainSelectOption) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	attrs := append(elem.ApplyParams(params), htmlio.Attr("value", opt.value))
	return htmlio.WriteHTMLInner(w, elem.Jid(), "option", "", template.HTML(template.HTMLEscapeString(opt.label)), attrs...)
}

func (plainSelectOption) JawsUpdate(elem *jaws.Element) {}

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

func TestSelectWidget_AppendsOptionBeforeSettingNewValue(t *testing.T) {
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

	opt1 := plainSelectOption{value: "1", label: "one"}
	opt2 := plainSelectOption{value: "2", label: "two"}
	sh := &testSelectHandler{
		testContainer: &testContainer{contents: []jaws.UI{opt1}},
		testSetter:    newTestSetter("1"),
	}
	selectUI := NewSelect(sh)
	elem := tr.NewElement(selectUI)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}

	sh.Set("2")
	sh.contents = []jaws.UI{opt1, opt2}
	selectUI.JawsUpdate(elem)
	tr.InCh <- wire.WsMsg{}

	sawAppend := false
	for {
		select {
		case <-t.Context().Done():
			t.Fatal("no select value update received")
		case msg := <-tr.OutCh:
			switch msg.What {
			case what.Append:
				sawAppend = true
			case what.Value:
				if !sawAppend {
					t.Fatalf("select Value %q was queued before appending the option it selects", msg.Data)
				}
				if msg.Data != "2" {
					t.Fatalf("select Value = %q, want %q", msg.Data, "2")
				}
				return
			}
		}
	}
}

// TestSelectWidget_NonSetterContainer exercises Select's defensive guard. NewSelect
// always supplies a named.SelectHandler (a bind.Setter[string]), so the false branch
// in JawsUpdate/JawsInput is reachable only by reassigning the embedded Container
// field to a plain jaws.Container. JawsUpdate must then update only the child options
// (no value set, no panic) and JawsInput must be a no-op returning nil.
func TestSelectWidget_NonSetterContainer(t *testing.T) {
	_, rq := newCoreRequest(t)
	c := &testContainer{contents: []jaws.UI{NewOption(named.NewBool(nil, "1", "one", true))}}
	selectUI := &Select{ContainerHelper: NewContainerHelper(c)}
	elem, _ := renderUI(t, rq, selectUI)

	selectUI.JawsUpdate(elem)

	if err := selectUI.JawsInput(elem, "x"); err != nil {
		t.Fatalf("JawsInput on non-Setter container: want nil, got %v", err)
	}
}
