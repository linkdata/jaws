package ui

import (
	"errors"
	"fmt"
	"html/template"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/named"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// The templates used by the ownership tests. Every nested helper passes $.Dot along,
// so one dot tags the whole subtree and GetElements(dot) counts it.
const ownedTestTemplates = `
{{define "owned-parent"}}{{$.RequestWriter.Template "" "owned-leaf" $.Dot}}{{end}}
{{define "owned-leaf"}}leaf{{end}}
{{define "owned-deep"}}{{$.RequestWriter.Template "" "owned-parent" $.Dot}}{{end}}
{{define "owned-failafter"}}{{$.RequestWriter.Template "" "owned-leaf" $.Dot}}{{$.Dot.Check}}{{end}}
{{define "owned-many"}}{{range $.Dot.Names}}{{$.RequestWriter.Template "" "owned-leaf" $.Dot}}{{end}}{{end}}
{{define "owned-container"}}{{$.RequestWriter.Container "div" $.Dot.Container}}{{end}}
{{define "owned-register"}}<div id="{{$.RequestWriter.Register $.Dot}}"></div>{{end}}
{{define "owned-radiogroup"}}{{range $.RequestWriter.RadioGroup $.Dot.Radios}}{{.Radio}}{{.Label}}{{end}}{{end}}
{{define "owned-register-discarded"}}registered as text: {{$.RequestWriter.Register $.Dot}}{{end}}
{{define "owned-labelonly"}}{{range $.RequestWriter.RadioGroup $.Dot.Radios}}{{.Label}}{{end}}{{end}}
{{define "owned-register-failafter"}}<div id="{{$.RequestWriter.Register $.Dot}}"></div>{{$.Dot.Check}}{{end}}
{{define "owned-radiogroup-failafter"}}{{range $.RequestWriter.RadioGroup $.Dot.Radios}}{{.Radio}}{{.Label}}{{end}}{{$.Dot.Check}}{{end}}
{{define "owned-radio-outer"}}{{$.RequestWriter.Template "div" "owned-radio-inner" ($.Dot.Box $.RequestWriter)}}{{end}}
{{define "owned-radio-inner"}}{{range $.Dot.Elements}}{{.Radio}}{{.Label}}{{end}}{{end}}
`

var errOwnedDotCheck = errors.New("owned dot check failed")

// ownedDot is the template data for the ownership tests. A pointer is usable as a
// tag, and Check fails template execution after nested UI has already rendered.
type ownedDot struct {
	mu        sync.Mutex
	fail      error
	container *testContainer
	names     []string
	radios    *named.BoolArray
	box       *ownedRadioBox
}

func (d *ownedDot) Check() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return "checked", d.fail
}

func (d *ownedDot) setFail(err error) {
	d.mu.Lock()
	d.fail = err
	d.mu.Unlock()
}

func (d *ownedDot) Container() jaws.Container { return d.container }

func (d *ownedDot) Names() []string { return d.names }

func (d *ownedDot) Radios() *named.BoolArray { return d.radios }

// Box builds the radio group with the passed-in writer — the outer template's — and
// returns the box carrying the RadioElement values to the nested template, whose dot it
// becomes. It is a pointer so it is usable as a tag.
func (d *ownedDot) Box(rw RequestWriter) *ownedRadioBox {
	d.box.rel = rw.RadioGroup(d.radios)
	return d.box
}

// ownedRadioBox carries RadioElement values across a template boundary, so a test can
// render them in a nested template while the outer template owns their Elements.
type ownedRadioBox struct {
	rel   []RadioElement
	show  bool
	execs int // executions of the nested template that read Elements
}

// Elements returns the group to render, or nothing once show is cleared. It counts
// calls so a test can tell an update that ran and rendered nothing from one that never
// executed.
func (b *ownedRadioBox) Elements() []RadioElement {
	b.execs++
	if !b.show {
		return nil
	}
	return b.rel
}

// JawsUpdate makes ownedDot usable as the $.Register updater. Register tags its
// Element with the updater, so the dot still tags the whole subtree.
func (d *ownedDot) JawsUpdate(*jaws.Element) {}

func newOwnedLookuper(t *testing.T) *template.Template {
	t.Helper()
	return template.Must(template.New("owned").Parse(ownedTestTemplates))
}

// newOwnedRequest returns a Jaws with the ownership test templates registered and a
// request to render them into.
func newOwnedRequest(t *testing.T) (*jaws.Jaws, *jaws.Request) {
	t.Helper()
	jw, rq := newCoreRequest(t)
	if err := jw.AddTemplateLookuper(newOwnedLookuper(t)); err != nil {
		t.Fatal(err)
	}
	return jw, rq
}

// maxProbedJid bounds the Jid probe in countRegistered. The ownership tests create
// well under this many Elements.
const maxProbedJid = jaws.Jid(500)

// countRegistered returns how many Elements are still registered in rq, probing the
// Jid space directly so Elements carrying no tag of their own (unwrapped nested
// templates, container children) are counted too.
func countRegistered(t *testing.T, rq *jaws.Request) (count int) {
	t.Helper()
	for jid := jaws.Jid(1); jid <= maxProbedJid; jid++ {
		if rq.GetElementByJid(jid) != nil {
			count++
		}
	}
	return
}

// renderOwned renders ui into a new Element and returns it.
func renderOwned(t *testing.T, rq *jaws.Request, ui jaws.UI) *jaws.Element {
	t.Helper()
	elem := rq.NewElement(ui)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}
	return elem
}

// TestTemplate_NestedUnwrappedDoesNotLeakOnUpdate is the reported issue (#216): a
// wrapped template invoking an unwrapped nested one must not register an extra
// Element on every update.
func TestTemplate_NestedUnwrappedDoesNotLeakOnUpdate(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	if err = jw.AddTemplateLookuper(newOwnedLookuper(t)); err != nil {
		t.Fatal(err)
	}
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	t.Cleanup(func() {
		tr.Close()
		<-tr.DoneCh
	})
	<-tr.ReadyCh

	dot := &ownedDot{}
	rw := RequestWriter{Request: tr.Request, Writer: tr.Recorder}
	if err = rw.Template("div", "owned-parent", dot); err != nil {
		t.Fatal(err)
	}
	if got := len(tr.GetElements(dot)); got != 2 {
		t.Fatalf("initial tagged elements = %d, want 2 (wrapper and nested)", got)
	}

	for round := 1; round <= 3; round++ {
		tr.BcastCh <- wire.Message{Dest: dot, What: what.Update}
		select {
		case msg := <-tr.OutCh:
			if msg.What != what.Inner {
				t.Fatalf("round %d: queued %v, want %v", round, msg.What, what.Inner)
			}
		case <-time.After(time.Second):
			t.Fatalf("round %d: timeout waiting for parent update", round)
		}
		if got := len(tr.GetElements(dot)); got != 2 {
			t.Fatalf("tagged elements after %d update(s) = %d, want 2", round, got)
		}
	}
}

// TestTemplate_UpdateReplacesOwnedGeneration checks that the surviving nested
// Element is the one the update created, and the replaced one is unregistered.
func TestTemplate_UpdateReplacesOwnedGeneration(t *testing.T) {
	_, rq := newOwnedRequest(t)

	dot := &ownedDot{}
	tmpl := NewTemplate("div", "owned-parent", dot)
	elem := renderOwned(t, rq, tmpl)

	first := tr1Nested(t, rq, dot, elem)
	tmpl.JawsUpdate(elem)
	second := tr1Nested(t, rq, dot, elem)

	if first == second {
		t.Fatal("update reused the previous nested Element instead of rendering a new one")
	}
	if !first.Deleted() {
		t.Error("replaced nested Element does not report Deleted")
	}
	if rq.GetElementByJid(first.Jid()) != nil {
		t.Error("replaced nested Element is still registered")
	}
}

// tr1Nested returns the single nested Element tagged with dot, failing if the
// wrapper is not accompanied by exactly one.
func tr1Nested(t *testing.T, rq *jaws.Request, dot *ownedDot, wrapper *jaws.Element) *jaws.Element {
	t.Helper()
	var nested []*jaws.Element
	for _, elem := range rq.GetElements(dot) {
		if elem != wrapper {
			nested = append(nested, elem)
		}
	}
	if len(nested) != 1 {
		t.Fatalf("nested elements = %d, want 1", len(nested))
	}
	return nested[0]
}

// TestTemplate_UpdateExecuteFailureKeepsPreviousGeneration checks the rollback: a
// failed update discards the Elements it created, keeps the previous ones (their DOM
// is untouched), and a later successful update reclaims exactly those.
func TestTemplate_UpdateExecuteFailureKeepsPreviousGeneration(t *testing.T) {
	jw, rq := newOwnedRequest(t)
	logger := new(templateLogger)
	jw.Logger = logger

	dot := &ownedDot{}
	tmpl := NewTemplate("div", "owned-failafter", dot)
	elem := renderOwned(t, rq, tmpl)
	first := tr1Nested(t, rq, dot, elem)

	dot.setFail(errOwnedDotCheck)
	tmpl.JawsUpdate(elem)

	if len(logger.errors) != 1 || !errors.Is(logger.errors[0], errOwnedDotCheck) {
		t.Fatalf("logged errors = %v, want one %v", logger.errors, errOwnedDotCheck)
	}
	if got := tr1Nested(t, rq, dot, elem); got != first {
		t.Fatal("failed update did not keep the previous nested Element")
	}
	if first.Deleted() {
		t.Fatal("failed update deleted the previous nested Element whose DOM is still live")
	}

	// The previous generation must still be owned, so a later success reclaims it
	// rather than leaking it.
	dot.setFail(nil)
	tmpl.JawsUpdate(elem)
	if !first.Deleted() {
		t.Error("the following successful update did not reclaim the previous nested Element")
	}
	if got := len(rq.GetElements(dot)); got != 2 {
		t.Errorf("tagged elements after recovery = %d, want 2", got)
	}
}

// TestTemplate_UpdateLookupFailureKeepsPreviousGeneration covers the update path
// that returns before the owned set is detached.
func TestTemplate_UpdateLookupFailureKeepsPreviousGeneration(t *testing.T) {
	jw, rq := newCoreRequest(t)
	logger := new(templateLogger)
	jw.Logger = logger
	lookuper := newOwnedLookuper(t)
	if err := jw.AddTemplateLookuper(lookuper); err != nil {
		t.Fatal(err)
	}

	dot := &ownedDot{}
	tmpl := NewTemplate("div", "owned-parent", dot)
	elem := renderOwned(t, rq, tmpl)
	first := tr1Nested(t, rq, dot, elem)

	if err := jw.RemoveTemplateLookuper(lookuper); err != nil {
		t.Fatal(err)
	}
	tmpl.JawsUpdate(elem)
	if len(logger.errors) != 1 || !errors.Is(logger.errors[0], ErrMissingTemplate) {
		t.Fatalf("logged errors = %v, want one %v", logger.errors, ErrMissingTemplate)
	}
	if got := tr1Nested(t, rq, dot, elem); got != first || first.Deleted() {
		t.Fatal("lookup failure disturbed the previous nested Element")
	}

	if err := jw.AddTemplateLookuper(lookuper); err != nil {
		t.Fatal(err)
	}
	tmpl.JawsUpdate(elem)
	if !first.Deleted() {
		t.Error("the following successful update did not reclaim the previous nested Element")
	}
}

// TestTemplate_RenderFailureDeletesOwnedElements checks that an execution error
// during the initial render leaves nothing registered: NewUI drops the wrapper and
// the template drops the nested UI it had already created.
func TestTemplate_RenderFailureDeletesOwnedElements(t *testing.T) {
	_, rq := newOwnedRequest(t)

	dot := &ownedDot{fail: errOwnedDotCheck}
	var sb strings.Builder
	rw := RequestWriter{Request: rq, Writer: &sb}
	if err := rw.Template("div", "owned-failafter", dot); !errors.Is(err, errOwnedDotCheck) {
		t.Fatalf("render error = %v, want %v", err, errOwnedDotCheck)
	}
	if !strings.Contains(sb.String(), "leaf") {
		t.Fatalf("nested UI did not render before the failure: %q", sb.String())
	}
	if got := countRegistered(t, rq); got != 0 {
		t.Fatalf("registered elements after failed render = %d, want 0", got)
	}
}

// failOnPayload fails the write whose payload matches exactly, so a test can fail
// the wrapper's closing tag without counting writes.
type failOnPayload struct {
	payload string
	err     error
	sb      strings.Builder
}

func (w *failOnPayload) Write(p []byte) (int, error) {
	if string(p) == w.payload {
		return 0, w.err
	}
	return w.sb.Write(p)
}

// TestTemplate_RenderClosingWriteFailureDeletesOwnedElements mirrors
// TestRequestWriterUI_ContainerClosingWriteErrorDoesNotLeakChildren: the nested UI
// renders, then the wrapper's closing tag fails to write.
func TestTemplate_RenderClosingWriteFailureDeletesOwnedElements(t *testing.T) {
	_, rq := newOwnedRequest(t)

	writeErr := errors.New("closing write failed")
	writer := &failOnPayload{payload: "</div>", err: writeErr}
	dot := &ownedDot{}
	rw := RequestWriter{Request: rq, Writer: writer}
	if err := rw.Template("div", "owned-parent", dot); !errors.Is(err, writeErr) {
		t.Fatalf("render error = %v, want %v", err, writeErr)
	}
	if !strings.Contains(writer.sb.String(), "leaf") {
		t.Fatalf("nested UI did not render before the failure: %q", writer.sb.String())
	}
	if got := countRegistered(t, rq); got != 0 {
		t.Fatalf("registered elements after failed closing write = %d, want 0", got)
	}
}

// TestRequestWriter_NewUIReportsElementBeforeRendering checks that NewUI reports the
// Element it created even when the render then fails, and unregisters it regardless.
// Reporting at creation is what lets an owner reclaim an Element that never rendered.
func TestRequestWriter_NewUIReportsElementBeforeRendering(t *testing.T) {
	_, rq := newOwnedRequest(t)

	var seen []*jaws.Element
	var sb strings.Builder
	rw := RequestWriter{
		Request: rq,
		Writer:  &sb,
		elementCreated: func(elem *jaws.Element) {
			seen = append(seen, elem)
		},
	}
	// The template renders nested UI and then fails, so this writer creates one
	// Element (the wrapper) and the nested writer inside it creates its own.
	dot := &ownedDot{fail: errOwnedDotCheck}
	if err := rw.Template("div", "owned-failafter", dot); !errors.Is(err, errOwnedDotCheck) {
		t.Fatalf("render error = %v, want %v", err, errOwnedDotCheck)
	}
	if len(seen) != 1 {
		t.Fatalf("hook calls = %d, want 1 (only Elements created directly through this writer)", len(seen))
	}
	if !seen[0].Deleted() {
		t.Error("the reported Element was not unregistered after its render failed")
	}
	if got := countRegistered(t, rq); got != 0 {
		t.Fatalf("registered elements after failed render = %d, want 0", got)
	}
}

// TestTemplate_UpdateReclaimsWholeSubtree covers the recursive walk: the wrapper
// owns a nested unwrapped template that owns another one.
func TestTemplate_UpdateReclaimsWholeSubtree(t *testing.T) {
	_, rq := newOwnedRequest(t)

	dot := &ownedDot{}
	tmpl := NewTemplate("div", "owned-deep", dot)
	elem := renderOwned(t, rq, tmpl)
	if got := countRegistered(t, rq); got != 3 {
		t.Fatalf("registered elements after render = %d, want 3", got)
	}

	for round := 1; round <= 3; round++ {
		tmpl.JawsUpdate(elem)
		if got := countRegistered(t, rq); got != 3 {
			t.Fatalf("registered elements after %d update(s) = %d, want 3", round, got)
		}
	}
}

// TestTemplate_UpdateReclaimsNestedContainer covers a container nested in a
// template: the replaced container Element takes its children with it.
func TestTemplate_UpdateReclaimsNestedContainer(t *testing.T) {
	_, rq := newOwnedRequest(t)

	tc := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("a")), NewSpan(testHTMLGetter("b"))}}
	dot := &ownedDot{container: tc}
	tmpl := NewTemplate("div", "owned-container", dot)
	elem := renderOwned(t, rq, tmpl)
	// wrapper, container, two spans
	if got := countRegistered(t, rq); got != 4 {
		t.Fatalf("registered elements after render = %d, want 4", got)
	}

	for round := 1; round <= 3; round++ {
		tmpl.JawsUpdate(elem)
		if got := countRegistered(t, rq); got != 4 {
			t.Fatalf("registered elements after %d update(s) = %d, want 4", round, got)
		}
	}
}

// TestContainer_RemovedChildReleasesOwnedElements covers the other nesting
// direction: a container child that owns nested UI releases it when removed.
func TestContainer_RemovedChildReleasesOwnedElements(t *testing.T) {
	_, rq := newOwnedRequest(t)

	dot := &ownedDot{}
	keep := NewTemplate("div", "owned-parent", dot)
	drop := NewTemplate("div", "owned-parent", dot)
	tc := &testContainer{contents: []jaws.UI{keep, drop}}
	container := NewContainer("div", tc)
	elem := renderOwned(t, rq, container)
	// container, two child templates, one nested Element each
	if got := countRegistered(t, rq); got != 5 {
		t.Fatalf("registered elements after render = %d, want 5", got)
	}

	tc.contents = []jaws.UI{keep}
	container.JawsUpdate(elem)
	if got := countRegistered(t, rq); got != 3 {
		t.Fatalf("registered elements after removing a child = %d, want 3", got)
	}
}

// TestContainer_FailedAppendReleasesOwnedElementsBeforePanic checks that the failed
// child and its nested UI are unregistered before MustLog panics for want of a
// configured logger.
func TestContainer_FailedAppendReleasesOwnedElementsBeforePanic(t *testing.T) {
	_, rq := newOwnedRequest(t)

	tc := &testContainer{}
	container := NewContainer("div", tc)
	elem := renderOwned(t, rq, container)
	before := countRegistered(t, rq)

	dot := &ownedDot{fail: errOwnedDotCheck}
	tc.contents = []jaws.UI{NewTemplate("div", "owned-failafter", dot)}
	recovered := func() (recovered any) {
		defer func() { recovered = recover() }()
		container.JawsUpdate(elem)
		return
	}()
	if err, ok := recovered.(error); !ok || !errors.Is(err, errOwnedDotCheck) {
		t.Fatalf("recovered = %v, want a panic wrapping %v", recovered, errOwnedDotCheck)
	}
	if got := countRegistered(t, rq); got != before {
		t.Fatalf("registered elements after failed append = %d, want %d", got, before)
	}
}

// TestPageTemplate_RenderFailureDeletesOwnedElements covers the page template's
// error path, which has no update to reclaim anything later.
func TestPageTemplate_RenderFailureDeletesOwnedElements(t *testing.T) {
	_, rq := newOwnedRequest(t)

	dot := &ownedDot{fail: errOwnedDotCheck}
	var sb strings.Builder
	rw := RequestWriter{Request: rq, Writer: &sb}
	err := rw.NewUI(&pageTemplate{Template: Template{Name: "owned-failafter", Dot: dot}})
	if !errors.Is(err, errOwnedDotCheck) {
		t.Fatalf("render error = %v, want %v", err, errOwnedDotCheck)
	}
	if got := countRegistered(t, rq); got != 0 {
		t.Fatalf("registered elements after failed page render = %d, want 0", got)
	}
}

// TestTemplate_UpdateTracksRegisterAndRadioGroup covers the two helpers that create
// their Elements through Request.NewElement rather than RequestWriter.NewUI: they
// report them to the writer's owner, so the counts stay flat across updates with no
// client attached to acknowledge DOM removals.
//
// The label-only case is why ownership is recorded at creation: RadioElement.Label
// creates the radio Element for its for= attribute without ever rendering it, so an
// after-render notification would miss it and it would accumulate.
func TestTemplate_UpdateTracksRegisterAndRadioGroup(t *testing.T) {
	newRadios := func() *named.BoolArray {
		nba := named.NewBoolArray(false)
		nba.Add("1", "one")
		return nba
	}

	for _, tt := range []struct {
		name     string
		template string
		dot      *ownedDot
		perRound int // Elements the helper creates per execution
	}{
		{"register", "owned-register", &ownedDot{}, 1},
		{"register discarded jid", "owned-register-discarded", &ownedDot{}, 1},
		{"radiogroup", "owned-radiogroup", &ownedDot{radios: newRadios()}, 2},           // input and label
		{"radiogroup label only", "owned-labelonly", &ownedDot{radios: newRadios()}, 2}, // unrendered radio, label
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, rq := newOwnedRequest(t)
			tmpl := NewTemplate("div", tt.template, tt.dot)
			elem := renderOwned(t, rq, tmpl)

			want := 1 + tt.perRound // the wrapper, plus this execution's
			if got := countRegistered(t, rq); got != want {
				t.Fatalf("registered elements after render = %d, want %d", got, want)
			}
			for round := 1; round <= 3; round++ {
				tmpl.JawsUpdate(elem)
				if got := countRegistered(t, rq); got != want {
					t.Fatalf("registered elements after %d update(s) = %d, want %d", round, got, want)
				}
			}
		})
	}
}

// TestTemplate_UpdateFailureKeepsRegisterAndRadioGroupGeneration covers the remaining
// leak mode: execution failing after either helper has already created its Elements.
// The failed execution's output is discarded, so its Elements must go and the previous
// generation must stay live to match the unchanged DOM.
func TestTemplate_UpdateFailureKeepsRegisterAndRadioGroupGeneration(t *testing.T) {
	newRadios := func() *named.BoolArray {
		nba := named.NewBoolArray(false)
		nba.Add("1", "one")
		return nba
	}

	for _, tt := range []struct {
		name     string
		template string
		dot      *ownedDot
		perRound int
	}{
		{"register", "owned-register-failafter", &ownedDot{}, 1},
		{"radiogroup", "owned-radiogroup-failafter", &ownedDot{radios: newRadios()}, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			jw, rq := newOwnedRequest(t)
			logger := new(templateLogger)
			jw.Logger = logger

			tmpl := NewTemplate("div", tt.template, tt.dot)
			elem := renderOwned(t, rq, tmpl)
			want := 1 + tt.perRound
			if got := countRegistered(t, rq); got != want {
				t.Fatalf("registered elements after render = %d, want %d", got, want)
			}
			first := registeredJids(t, rq)

			tt.dot.setFail(errOwnedDotCheck)
			tmpl.JawsUpdate(elem)
			if len(logger.errors) != 1 || !errors.Is(logger.errors[0], errOwnedDotCheck) {
				t.Fatalf("logged errors = %v, want one %v", logger.errors, errOwnedDotCheck)
			}
			if got := registeredJids(t, rq); !slices.Equal(got, first) {
				t.Fatalf("registered jids after failed update = %v, want the previous generation %v", got, first)
			}

			// The previous generation must still be owned, so the next success reclaims
			// it rather than leaking it.
			tt.dot.setFail(nil)
			tmpl.JawsUpdate(elem)
			if got := countRegistered(t, rq); got != want {
				t.Fatalf("registered elements after recovery = %d, want %d", got, want)
			}
			if got := registeredJids(t, rq); slices.Equal(got, first) {
				t.Fatal("the recovering update did not replace the previous generation")
			}
		})
	}
}

// TestRadioGroup_OwnedByTheTemplateThatCalledIt pins where ownership is attributed
// when a group's markup lands in a different wrapper than the RadioGroup call: the
// outer template calls $.RadioGroup and passes the RadioElement values to a nested
// wrapped template through its dot.
//
// Updating the outer template alone would not distinguish the two candidates, since
// the cleanup walk recurses into the nested Template's own owned set either way. The
// discriminating step is updating the nested Element on its own: with the radios no
// longer rendered and no client to acknowledge the DOM removal, they must still be
// registered, which is what fails if the nested template owned them.
func TestRadioGroup_OwnedByTheTemplateThatCalledIt(t *testing.T) {
	_, rq := newOwnedRequest(t)

	radios := named.NewBoolArray(false)
	radios.Add("1", "one")
	box := &ownedRadioBox{show: true}
	dot := &ownedDot{radios: radios, box: box}
	outer := NewTemplate("div", "owned-radio-outer", dot)
	outerElem := renderOwned(t, rq, outer)

	// outer wrapper, nested wrapper, radio, label
	const want = 4
	if got := countRegistered(t, rq); got != want {
		t.Fatalf("registered elements after render = %d, want %d", got, want)
	}
	inner := rq.GetElements(box)
	if len(inner) != 1 {
		t.Fatalf("nested wrapper elements = %d, want 1", len(inner))
	}

	// The nested template updates on its own and stops rendering the group. Counting
	// its executions keeps the assertion below from passing vacuously: the radios must
	// survive an update that ran and dropped them, not one that never happened.
	box.show = false
	execsBefore := box.execs
	inner[0].JawsUpdate()
	if box.execs != execsBefore+1 {
		t.Fatalf("nested template executions = %d, want %d: the nested update did not run",
			box.execs, execsBefore+1)
	}
	if got := countRegistered(t, rq); got != want {
		t.Fatalf("registered elements after the nested update = %d, want %d: the radio and label "+
			"belong to the template that called RadioGroup, so the nested update must not reclaim them", got, want)
	}

	// The owner reclaims them, along with the nested wrapper it also owns.
	outer.JawsUpdate(outerElem)
	if got := countRegistered(t, rq); got != 2 {
		t.Fatalf("registered elements after the outer update = %d, want 2 (both wrappers, no group)", got)
	}
}

// registeredJids returns the Jids still registered in rq, in ascending order.
func registeredJids(t *testing.T, rq *jaws.Request) (jids []jaws.Jid) {
	t.Helper()
	for jid := jaws.Jid(1); jid <= maxProbedJid; jid++ {
		if rq.GetElementByJid(jid) != nil {
			jids = append(jids, jid)
		}
	}
	return
}

// TestTemplate_UpdateReclaimsManyTaggedDescendants exercises the batched
// unregister with a generation large enough to span several tag entries.
func TestTemplate_UpdateReclaimsManyTaggedDescendants(t *testing.T) {
	_, rq := newOwnedRequest(t)

	const nested = 50
	names := make([]string, nested)
	for i := range names {
		names[i] = fmt.Sprintf("row-%d", i)
	}
	dot := &ownedDot{names: names}
	tmpl := NewTemplate("div", "owned-many", dot)
	// Several extra tags on the wrapper, so removing a generation has to visit more
	// than one tag entry.
	elem := rq.NewElement(tmpl)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, []any{tag.Tag("alpha"), tag.Tag("beta"), tag.Tag("gamma")}); err != nil {
		t.Fatal(err)
	}

	want := countRegistered(t, rq)
	if want != nested+1 {
		t.Fatalf("registered elements after render = %d, want %d", want, nested+1)
	}
	for round := 1; round <= 3; round++ {
		tmpl.JawsUpdate(elem)
		if got := countRegistered(t, rq); got != want {
			t.Fatalf("registered elements after %d update(s) = %d, want %d", round, got, want)
		}
		if got := len(rq.GetElements(dot)); got != want {
			t.Fatalf("tagged elements after %d update(s) = %d, want %d", round, got, want)
		}
	}
	for _, tagValue := range []tag.Tag{"alpha", "beta", "gamma"} {
		if got := rq.GetElements(tagValue); len(got) != 1 || got[0] != elem {
			t.Errorf("tag %q resolves to %v, want only the wrapper", tagValue, got)
		}
	}
}
