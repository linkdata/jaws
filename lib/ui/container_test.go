package ui

import (
	"context"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/named"
	"github.com/linkdata/jaws/lib/tag"
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
	if want := NewContainer("tbody", tc); tbody.Container != want {
		t.Fatalf("Tbody Container = %#v, want %#v", tbody.Container, want)
	}
	elem, got := renderUI(t, rq, tbody)
	mustMatch(t, `^<tbody id="Jid\.[0-9]+"><span id="Jid\.[0-9]+">foo</span><span id="Jid\.[0-9]+">bar</span></tbody>$`, got)
	tbody.JawsUpdate(elem)
}

func TestContainerUpdate(t *testing.T) {
	_, rq := newCoreRequest(t)
	span1 := NewSpan(testHTMLGetter("span1"))
	span2 := NewSpan(testHTMLGetter("span2"))
	span3 := NewSpan(testHTMLGetter("span3"))

	tc := &testContainer{contents: []jaws.UI{span1}}
	container := NewContainer("div", tc)
	elem, _ := renderUI(t, rq, container)

	if contents := containerElements(t, elem); len(contents) != 1 {
		t.Fatalf("want 1 content got %d", len(contents))
	}

	// append + reorder path
	tc.contents = []jaws.UI{span1, span2, span3}
	elem.JawsUpdate()
	if contents := containerElements(t, elem); len(contents) != 3 {
		t.Fatalf("want 3 contents got %d", len(contents))
	}

	// remove path
	removedJid := containerElements(t, elem)[0].Jid()
	tc.contents = []jaws.UI{span2, span3}
	elem.JawsUpdate()
	if got := rq.GetElementByJid(removedJid); got != nil {
		t.Fatal("expected removed element to be deleted from request")
	}

	// reorder + replace path
	tc.contents = []jaws.UI{span3, span1}
	elem.JawsUpdate()
	if contents := containerElements(t, elem); len(contents) != 2 {
		t.Fatalf("want 2 contents got %d", len(contents))
	}
}

// TestContainer_UpdateEmitsWireOps pins the browser-visible wire output of an
// update: appending a child must emit an Append carrying that child's
// rendered HTML and an Order reflecting the new sequence. Asserting the ops (not
// just the in-memory contents slice) catches regressions that line coverage misses.
func TestContainer_UpdateEmitsWireOps(t *testing.T) {
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
	elem.JawsUpdate()
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

func TestContainer_AppendsSelectBeforeSettingInitialValue(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	t.Cleanup(func() {
		tr.Close()
		<-tr.DoneCh
	})
	<-tr.ReadyCh

	outerHandler := &testContainer{}
	outer := NewContainer("div", outerHandler)
	outerElem := tr.NewElement(outer)
	var sb strings.Builder
	if err := outerElem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}

	selectHandler := &testSelectHandler{
		testContainer: &testContainer{contents: []jaws.UI{
			plainSelectOption{value: "1", label: "one"},
			plainSelectOption{value: "2", label: "two"},
		}},
		testSetter: newTestSetter("2"),
	}
	outerHandler.contents = []jaws.UI{NewSelect(selectHandler)}
	tr.BcastCh <- wire.Message{Dest: outerHandler, What: what.Update}

	sawAppend := false
	for {
		select {
		case msg := <-tr.OutCh:
			switch msg.What {
			case what.Append:
				sawAppend = true
				if !strings.Contains(msg.Data, "<select") {
					t.Fatalf("Append data %q does not contain the select", msg.Data)
				}
			case what.Value:
				if !sawAppend {
					t.Fatalf("select Value %q was sent before its containing Append", msg.Data)
				}
				if msg.Data != "2" {
					t.Fatalf("select Value = %q, want %q", msg.Data, "2")
				}
				return
			}
		case <-time.After(time.Second):
			t.Fatal("no appended select value update received")
		}
	}
}

func TestContainerUpdateDuplicates(t *testing.T) {
	_, rq := newCoreRequest(t)
	span1 := NewSpan(testHTMLGetter("span1"))
	span2 := NewSpan(testHTMLGetter("span2"))

	// render with duplicate UI
	tc := &testContainer{contents: []jaws.UI{span1, span2, span1}}
	container := NewContainer("div", tc)
	elem, _ := renderUI(t, rq, container)

	contents := containerElements(t, elem)
	if len(contents) != 3 {
		t.Fatalf("want 3 contents got %d", len(contents))
	}
	// the two span1 Elements must have distinct Jids
	jid0 := contents[0].Jid()
	jid2 := contents[2].Jid()
	if jid0 == jid2 {
		t.Fatal("duplicate UI must produce distinct Jids")
	}

	// remove one duplicate, keep the other
	tc.contents = []jaws.UI{span2, span1}
	elem.JawsUpdate()
	contents = containerElements(t, elem)
	if len(contents) != 2 {
		t.Fatalf("want 2 contents got %d", len(contents))
	}
	// one of the two span1 Jids should have been removed
	kept := contents[1].Jid()
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
	elem.JawsUpdate()
	contents = containerElements(t, elem)
	if len(contents) != 4 {
		t.Fatalf("want 4 contents got %d", len(contents))
	}
	// all four must have distinct Jids
	jids := make(map[jid.Jid]struct{}, 4)
	for i, c := range contents {
		if _, ok := jids[c.Jid()]; ok {
			t.Fatalf("contents[%d] has duplicate Jid %v", i, c.Jid())
		}
		jids[c.Jid()] = struct{}{}
	}
}

// TestContainer_ReconcileDiscardsOutOfBandDeletedChild verifies that a child
// Element deleted out-of-band (e.g. by a what.Delete broadcast on a shared tag, or a
// browser what.Remove) is not reused from the reconcile pool. A deleted Element is
// inert, so reusing it would leave the still-wanted child permanently unrendered and
// put a phantom Jid in the Order; reconcile must create a fresh Element instead.
func TestContainer_ReconcileDiscardsOutOfBandDeletedChild(t *testing.T) {
	_, rq := newCoreRequest(t)
	span1 := NewSpan(testHTMLGetter("span1"))
	span2 := NewSpan(testHTMLGetter("span2"))
	tc := &testContainer{contents: []jaws.UI{span1, span2}}
	container := NewContainer("div", tc)
	elem, _ := renderUI(t, rq, container)

	contents := containerElements(t, elem)
	if len(contents) != 2 {
		t.Fatalf("want 2 contents got %d", len(contents))
	}
	deletedChild := contents[1]
	deletedJid := deletedChild.Jid()

	// Delete span2's Element out-of-band while the container still wants it.
	rq.DeleteElement(deletedChild)
	if !deletedChild.Deleted() {
		t.Fatal("expected child to be marked deleted")
	}

	// tc.contents is unchanged (still wants span1 and span2), so reconcile must not
	// reuse the deleted Element for span2.
	elem.JawsUpdate()

	contents = containerElements(t, elem)
	if len(contents) != 2 {
		t.Fatalf("want 2 contents after update got %d", len(contents))
	}
	fresh := contents[1]
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

func TestContainer_SkipsOutOfBandDeletedLeftover(t *testing.T) {
	_, rq := newCoreRequest(t) // jaws.New(): nil Logger, so an invalid removal panics
	span1 := NewSpan(testHTMLGetter("span1"))
	span2 := NewSpan(testHTMLGetter("span2"))
	tc := &testContainer{contents: []jaws.UI{span1, span2}}
	container := NewContainer("div", tc)
	elem, _ := renderUI(t, rq, container)

	contents := containerElements(t, elem)
	if len(contents) != 2 {
		t.Fatalf("want 2 contents got %d", len(contents))
	}
	remainingChild := contents[0]
	deletedChild := contents[1]

	// Delete span2's Element out-of-band (as a what.Delete broadcast would via
	// rq.DeleteElement) and drop span2 from what the container wants. That routes the
	// deleted Element to the leftover path rather than the self-healing reuse path
	// exercised by TestContainer_ReconcileDiscardsOutOfBandDeletedChild.
	// Reconciliation must discard it without trying to remove it a second time.
	rq.DeleteElement(deletedChild)
	if !deletedChild.Deleted() {
		t.Fatal("expected child to be marked deleted")
	}
	tc.contents = []jaws.UI{span1}

	elem.JawsUpdate()

	contents = containerElements(t, elem)
	if len(contents) != 1 {
		t.Fatalf("want 1 content after update got %d", len(contents))
	}
	remaining := contents[0]
	if remaining != remainingChild {
		t.Fatal("remaining child must be reused")
	}
	if remaining.Deleted() || rq.GetElementByJid(remaining.Jid()) != remaining {
		t.Fatal("remaining child must be live and registered")
	}
}

func TestContainerRenderErrorPaths(t *testing.T) {
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
	if contents := containerElements(t, elem); len(contents) != 0 {
		t.Fatalf("want 0 successful child got %d", len(contents))
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
	elem2.JawsUpdate()
}

func TestContainerUpdateRenderErrorDoesNotAppendFailedChild(t *testing.T) {
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
	elem.JawsUpdate()

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

// BenchmarkContainerValidateChildren isolates the pre-lock validation scan that runs
// on every container render and update. It is the path affected by scoping the
// unusable-UI guard to the container: a usable child must cost only one
// self-comparison (a single deferred recover guards the whole slice) and no
// allocation. The broad Update benchmarks are dominated by rendering and allocation,
// so this focused benchmark is the meaningful guard for the scan's cost.
func BenchmarkContainerValidateChildren(b *testing.B) {
	b.ReportAllocs()
	children := benchChildren(0, 1000)
	for b.Loop() {
		if _, ok := firstUnusableChild(children); ok {
			b.Fatal("unexpected unusable child")
		}
	}
}

func BenchmarkContainerUpdateAppendHeavy(b *testing.B) {
	b.ReportAllocs()
	const size = 1000
	// Keep b.N because fixture cleanup leaves the timer stopped between iterations.
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
		elem.JawsUpdate()
		b.StopTimer()
		jw.Close()
	}
}

func BenchmarkContainerUpdateMixed(b *testing.B) {
	b.ReportAllocs()
	const size = 1000
	// Keep b.N because fixture cleanup leaves the timer stopped between iterations.
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
		elem.JawsUpdate()
		b.StopTimer()
		jw.Close()
	}
}

func TestContainerRenderErrorDoesNotLeakFailedChildElement(t *testing.T) {
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

type countingSelectHandler struct {
	*testSelectHandler
	getCount int
}

func (sh *countingSelectHandler) JawsGet(elem *jaws.Element) string {
	sh.getCount++
	return sh.testSetter.JawsGet(elem)
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

func TestSelectWidget_AppliesGetterAfterInitialRender(t *testing.T) {
	tests := []struct {
		name string
		sh   named.SelectHandler
		want string
	}{
		{
			name: "empty BoolArray selection",
			sh: named.NewBoolArray(false).
				Add("1", "one").
				Add("2", "two"),
		},
		{
			name: "custom getter differs from option markup",
			sh: &testSelectHandler{
				testContainer: &testContainer{contents: []jaws.UI{
					plainSelectOption{value: "1", label: "one"},
					plainSelectOption{value: "2", label: "two"},
				}},
				testSetter: newTestSetter("2"),
			},
			want: "2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jw, err := jaws.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(jw.Close)
			go jw.Serve()

			tr := jawstest.NewTestRequest(jw, nil)
			t.Cleanup(func() {
				tr.Close()
				<-tr.DoneCh
			})
			<-tr.ReadyCh

			rw := RequestWriter{Request: tr.Request, Writer: tr.Recorder}
			if err := rw.NewUI(NewSelect(tc.sh)); err != nil {
				t.Fatal(err)
			}
			if got := tr.BodyString(); strings.Contains(got, " selected") {
				t.Fatalf("initial option markup unexpectedly selected an option: %s", got)
			}

			tr.InCh <- wire.WsMsg{}
			select {
			case msg := <-tr.OutCh:
				if msg.What != what.Value || msg.Data != tc.want {
					t.Fatalf("initial select update = %+v, want Value %q", msg, tc.want)
				}
			case <-time.After(time.Second):
				t.Fatal("no initial select value update received")
			}
		})
	}
}

func TestSelectWidget_RenderErrorDoesNotApplyGetter(t *testing.T) {
	_, rq := newCoreRequest(t)
	renderErr := errors.New("render error")
	sh := &countingSelectHandler{testSelectHandler: &testSelectHandler{
		testContainer: &testContainer{contents: []jaws.UI{testRenderErrorUI{err: renderErr}}},
		testSetter:    newTestSetter("1"),
	}}
	var sb strings.Builder
	rw := RequestWriter{Request: rq, Writer: &sb}

	if err := rw.NewUI(NewSelect(sh)); !errors.Is(err, renderErr) {
		t.Fatalf("want %v got %v", renderErr, err)
	}
	if sh.getCount != 0 {
		t.Fatalf("getter called %d times after failed render", sh.getCount)
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
	tr.InCh <- wire.WsMsg{}
	select {
	case msg := <-tr.OutCh:
		if msg.What != what.Value || msg.Data != "1" {
			t.Fatalf("initial select update = %+v, want Value %q", msg, "1")
		}
	case <-time.After(time.Second):
		t.Fatal("no initial select value update received")
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

// nanChildUI is a comparable UI value that is not equal to itself (a non-reflexive
// map key), reproducing issue #179.
type nanChildUI struct{ f float64 }

func (nanChildUI) JawsRender(_ *jaws.Element, w io.Writer, _ []any) error {
	_, err := io.WriteString(w, "child")
	return err
}
func (nanChildUI) JawsUpdate(*jaws.Element) {}

// incomparableChildUI is statically comparable (an interface field) but panics when
// compared or hashed at runtime, since it holds a slice.
type incomparableChildUI struct{ v any }

func (incomparableChildUI) JawsRender(_ *jaws.Element, w io.Writer, _ []any) error {
	_, err := io.WriteString(w, "child")
	return err
}
func (incomparableChildUI) JawsUpdate(*jaws.Element) {}

// typedNilChildUI has pointer-receiver methods that tolerate a nil receiver, so a
// typed nil (*typedNilChildUI)(nil) is a usable, reflexive child.
type typedNilChildUI struct{}

func (*typedNilChildUI) JawsRender(_ *jaws.Element, w io.Writer, _ []any) error {
	_, err := io.WriteString(w, "child")
	return err
}
func (*typedNilChildUI) JawsUpdate(*jaws.Element) {}

// TestContainerAcceptsTypedNilChild documents that a typed nil child (a non-nil
// interface holding a nil pointer) is usable — comparable and equal to itself — so the
// container renders it and, because it is a stable reflexive map key, reuses the same
// Element across updates rather than churning (contrast a NaN-bearing child). Only a
// nil interface child is rejected.
func TestContainerAcceptsTypedNilChild(t *testing.T) {
	_, rq := newCoreRequest(t)
	tc := &testContainer{contents: []jaws.UI{(*typedNilChildUI)(nil)}}
	container := NewContainer("div", tc)
	elem, got := renderUI(t, rq, container)
	if cause := context.Cause(rq.Context()); cause != nil {
		t.Fatalf("render cancelled the Request: %v", cause)
	}
	if !strings.Contains(got, "child") {
		t.Fatalf("render = %q, want it to contain the child output", got)
	}
	contents := containerElements(t, elem)
	if len(contents) != 1 {
		t.Fatalf("want 1 child Element, got %d", len(contents))
	}
	childElem := contents[0]
	childJid := childElem.Jid()

	elem.JawsUpdate()
	if cause := context.Cause(rq.Context()); cause != nil {
		t.Fatalf("update cancelled the Request: %v", cause)
	}
	contents = containerElements(t, elem)
	if len(contents) != 1 {
		t.Fatalf("want 1 child Element after update, got %d", len(contents))
	}
	if contents[0] != childElem {
		t.Fatal("typed nil child was recreated on update instead of reused")
	}
	if got := contents[0].Jid(); got != childJid {
		t.Fatalf("child Jid changed on update: %v -> %v", childJid, got)
	}
}

// TestContainerValidatesWholeSliceBeforeRender pins that the pre-lock scan validates
// the entire children slice before any child is created, rendered, or committed: a
// usable child preceding an unusable one produces no Element, no output, and no
// committed content when the Request is terminated.
func TestContainerValidatesWholeSliceBeforeRender(t *testing.T) {
	_, rq := newCoreRequest(t)
	tc := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("VALIDMARKER")), nanChildUI{f: math.NaN()}}}
	container := NewContainer("div", tc)
	elem := rq.NewElement(container)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatalf("JawsRender err = %v, want nil", err)
	}
	if cause := context.Cause(rq.Context()); !errors.Is(cause, tag.ErrNotUsableAsTag) {
		t.Fatalf("cause = %v, want wrapping tag.ErrNotUsableAsTag", cause)
	}
	if strings.Contains(sb.String(), "VALIDMARKER") {
		t.Fatalf("valid prefix child was rendered before the slice was validated: %q", sb.String())
	}
	if contents := containerElements(t, elem); len(contents) != 0 {
		t.Fatalf("no children should be committed, got %d", len(contents))
	}
	// No child Element was ever created: NewElement is never reached. Jids are
	// monotonic (a created-then-deleted Element still advances the counter, and would
	// not be found by a registry lookup), so a probe created now must take the Jid
	// right after the container's; a higher Jid would mean a child was created.
	probe := rq.NewElement(NewSpan(testHTMLGetter("probe")))
	if probe.Jid() != elem.Jid()+1 {
		t.Fatalf("a child Element was created despite prevalidation abort: probe Jid %v, want %v", probe.Jid(), elem.Jid()+1)
	}
}

// TestContainerUpdateValidatesWholeSliceBeforeReconcile is the update-path counterpart:
// when a wanted slice has a usable child before an unusable one, the whole slice is
// validated before reconcile touches the state contents or the pool. It exercises
// reconcile directly and asserts its five operation sets are empty (so Container.update
// queues no Remove/Append/Order), that the contents are untouched, and — via a monotonic-Jid probe
// — that no new Element was created.
func TestContainerUpdateValidatesWholeSliceBeforeReconcile(t *testing.T) {
	_, rq := newCoreRequest(t)
	tc := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("OLD"))}}
	container := NewContainer("div", tc)
	elem, _ := renderUI(t, rq, container)
	contents := containerElements(t, elem)
	if len(contents) != 1 {
		t.Fatalf("want 1 child before update, got %d", len(contents))
	}
	childElem := contents[0]

	// Observe reconcile's outputs directly: a usable child before an unusable one must
	// yield no operations at all, which is what leaves Container.update nothing to queue.
	toAppend, toRemove, alreadyDeleted, oldOrder, newOrder := requireContainerState(t, elem).reconcile(elem,
		[]jaws.UI{NewSpan(testHTMLGetter("NEWVALID")), nanChildUI{f: math.NaN()}})
	if len(toAppend)+len(toRemove)+len(alreadyDeleted)+len(oldOrder)+len(newOrder) != 0 {
		t.Fatalf("reconcile produced operations on abort: append=%d remove=%d deleted=%d oldOrder=%d newOrder=%d",
			len(toAppend), len(toRemove), len(alreadyDeleted), len(oldOrder), len(newOrder))
	}
	if cause := context.Cause(rq.Context()); !errors.Is(cause, tag.ErrNotUsableAsTag) {
		t.Fatalf("cause = %v, want wrapping tag.ErrNotUsableAsTag", cause)
	}
	// No partial reconciliation: the existing content slice is untouched.
	contents = containerElements(t, elem)
	if len(contents) != 1 || contents[0] != childElem {
		t.Fatalf("contents changed on aborted update: partial reconciliation occurred")
	}
	// No new Element was created (Jids are monotonic, so a created-then-deleted child
	// would still have advanced the counter): a probe takes the Jid right after the
	// existing child's.
	probe := rq.NewElement(NewSpan(testHTMLGetter("probe")))
	if probe.Jid() != childElem.Jid()+1 {
		t.Fatalf("a child Element was created on aborted update: probe Jid %v, want %v", probe.Jid(), childElem.Jid()+1)
	}
}

// TestContainerTerminatesOnUnusableChild covers issue #179 and its
// runtime-incomparable sibling: a container child UI that is not equal to itself
// (holds NaN) or not comparable at runtime (holds an interface-wrapped slice) cannot
// be a container pool key. The container must terminate the Request rather than churn
// (NaN) or panic hashing the key (incomparable). The incomparable update case pins
// that reconcile validates before the pool lookup, which would otherwise panic.
func TestContainerTerminatesOnUnusableChild(t *testing.T) {
	bad := []struct {
		name string
		make func() jaws.UI
	}{
		{"nil", func() jaws.UI { return nil }},
		{"nan", func() jaws.UI { return nanChildUI{f: math.NaN()} }},
		{"incomparable", func() jaws.UI { return incomparableChildUI{v: []int{1}} }},
	}
	for _, b := range bad {
		t.Run(b.name+" render", func(t *testing.T) {
			_, rq := newCoreRequest(t)
			tc := &testContainer{contents: []jaws.UI{b.make()}}
			container := NewContainer("div", tc)
			elem := rq.NewElement(container)
			var sb strings.Builder
			// The render still balances its tags and returns nil; the unusable child is
			// skipped and the Request is terminated (asserted via the cancellation cause).
			if err := elem.JawsRender(&sb, nil); err != nil {
				t.Fatalf("JawsRender err = %v, want nil", err)
			}
			if cause := context.Cause(rq.Context()); !errors.Is(cause, tag.ErrNotUsableAsTag) {
				t.Fatalf("cause = %v, want wrapping tag.ErrNotUsableAsTag", cause)
			}
		})

		t.Run(b.name+" update", func(t *testing.T) {
			_, rq := newCoreRequest(t)
			tc := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("ok"))}}
			container := NewContainer("div", tc)
			elem, _ := renderUI(t, rq, container)
			tc.contents = []jaws.UI{b.make()}
			elem.JawsUpdate() // must terminate, not panic
			if cause := context.Cause(rq.Context()); !errors.Is(cause, tag.ErrNotUsableAsTag) {
				t.Fatalf("cause = %v, want wrapping tag.ErrNotUsableAsTag", cause)
			}
		})
	}
}

type reentrantLogger struct{ onError func() }

func (reentrantLogger) Info(string, ...any) {}
func (reentrantLogger) Warn(string, ...any) {}
func (l reentrantLogger) Error(string, ...any) {
	if l.onError != nil {
		l.onError()
	}
}

// TestContainerCancelNotUnderLock pins that terminating the Request for an unusable
// child does not run while the state mutex is held: cancelUnusableChildren runs before reconcile
// locks. Request.Cancel invokes the user logger synchronously, so a logger that
// re-enters the state lock would deadlock the update goroutine if cancellation held it.
// The timeout turns that regression into a failure instead of a hang.
func TestContainerCancelNotUnderLock(t *testing.T) {
	var st *containerState
	var reentered atomic.Bool
	_, rq := newConfiguredCoreRequest(t, func(jw *jaws.Jaws) {
		jw.Logger = reentrantLogger{onError: func() {
			// Re-enter the state lock. If the cancellation that triggered this log
			// held it, this second Lock on the same goroutine would deadlock.
			st.mu.Lock()
			_ = len(st.contents)
			st.mu.Unlock()
			reentered.Store(true)
		}}
	})
	tc := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("ok"))}}
	container := NewContainer("div", tc)
	elem, _ := renderUI(t, rq, container)
	st = requireContainerState(t, elem)

	tc.contents = []jaws.UI{nanChildUI{f: math.NaN()}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		elem.JawsUpdate()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("JawsUpdate deadlocked: Request cancellation ran while holding u.mu")
	}

	// Without this the deadlock check is vacuous: if the logger never ran, no re-entry
	// was attempted and the test would pass even with cancellation under the lock.
	if !reentered.Load() {
		t.Fatal("logger callback did not run; the deadlock check would be vacuous")
	}
	if cause := context.Cause(rq.Context()); !errors.Is(cause, tag.ErrNotUsableAsTag) {
		t.Fatalf("cause = %v, want wrapping tag.ErrNotUsableAsTag", cause)
	}
}
