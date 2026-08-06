package ui

import (
	"errors"
	"html/template"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

const templateStateTemplates = `
{{define "state-parent"}}{{$.RequestWriter.Template "div" "state-leaf" $.Dot}}{{end}}
{{define "state-leaf"}}leaf{{end}}
{{define "state-failafter"}}{{$.RequestWriter.Template "div" "state-leaf" $.Dot}}{{$.Dot.Check}}{{end}}
{{define "state-plain"}}plain{{end}}
{{define "state-b"}}b{{end}}
{{define "state-span"}}{{$.RequestWriter.Span "x"}}{{end}}
`

// clickCountingDot counts clicks delivered through a Template's event delegation.
type clickCountingDot struct{ clicks atomic.Int32 }

func (d *clickCountingDot) JawsClick(*jaws.Element, jaws.Click) error {
	d.clicks.Add(1)
	return nil
}

func newStateRequest(t *testing.T) (*jaws.Jaws, *jaws.Request) {
	t.Helper()
	return newConfiguredStateRequest(t, nil)
}

// newConfiguredStateRequest configures the Jaws before the Request exists; see
// newConfiguredCoreRequest for why that ordering matters.
func newConfiguredStateRequest(t *testing.T, configure func(*jaws.Jaws)) (*jaws.Jaws, *jaws.Request) {
	t.Helper()
	jw, rq := newConfiguredCoreRequest(t, configure)
	if err := jw.AddTemplateLookuper(template.Must(template.New("state").Parse(templateStateTemplates))); err != nil {
		t.Fatal(err)
	}
	return jw, rq
}

// ownedGeneration returns the Elements currently tracked for elem, which is the generation
// the next update will reclaim. It reads the slot's own mutex, like the Template does.
func ownedGeneration(t *testing.T, elem *jaws.Element) []*jaws.Element {
	t.Helper()
	st := templateStateOf(elem)
	if st == nil {
		t.Fatalf("element %v has no Template state", elem.Jid())
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]*jaws.Element(nil), st.owned...)
}

// TestTemplate_EqualValuesKeepIndependentGenerations covers what the state slot buys: one
// Template value backs several live Elements, each with its own generation.
//
// The assertions are on Element identity rather than on a registered count. A count alone
// cannot tell a correct update from one that reclaims the wrong wrapper's child and creates
// a replacement for it, since both leave the same number of Elements registered.
func TestTemplate_EqualValuesKeepIndependentGenerations(t *testing.T) {
	_, rq := newStateRequest(t)

	dot := &ownedDot{}
	tmpl := NewTemplate("div", "state-parent", dot)
	first := renderOwned(t, rq, tmpl)
	second := renderOwned(t, rq, NewTemplate("div", "state-parent", dot))

	// wrappers plus one nested Element each
	if got := countRegistered(t, rq); got != 4 {
		t.Fatalf("registered elements after two renders = %d, want 4", got)
	}

	// updateAndCheck updates one wrapper and asserts the replacement is scoped to it: its
	// own child is reclaimed and replaced, the other wrapper's child and tracked generation
	// are untouched. It returns the updated wrapper's fresh generation.
	updateAndCheck := func(name string, updated, other *jaws.Element, mine, theirs []*jaws.Element) []*jaws.Element {
		t.Helper()
		tmpl.JawsUpdate(updated)

		if !mine[0].Deleted() {
			t.Errorf("updating the %s wrapper left its previous child %v registered", name, mine[0].Jid())
		}
		if theirs[0].Deleted() {
			t.Fatalf("updating the %s wrapper deleted the other wrapper's child %v", name, theirs[0].Jid())
		}
		fresh := ownedGeneration(t, updated)
		if len(fresh) != 1 || fresh[0] == mine[0] {
			t.Fatalf("%s wrapper's generation after the update = %v, want one fresh Element", name, fresh)
		}
		if got := ownedGeneration(t, other); len(got) != 1 || got[0] != theirs[0] {
			t.Fatalf("the other wrapper's generation changed to %v, want %v", got, theirs[0].Jid())
		}
		if got := countRegistered(t, rq); got != 4 {
			t.Fatalf("registered elements after updating the %s wrapper = %d, want 4", name, got)
		}
		return fresh
	}

	firstGen, secondGen := ownedGeneration(t, first), ownedGeneration(t, second)
	if len(firstGen) != 1 || len(secondGen) != 1 {
		t.Fatalf("wrappers own %d and %d Elements, want 1 each", len(firstGen), len(secondGen))
	}
	if firstGen[0] == secondGen[0] {
		t.Fatal("the two wrappers share a nested Element; their generations are not independent")
	}

	firstGen = updateAndCheck("first", first, second, firstGen, secondGen)
	updateAndCheck("second", second, first, secondGen, firstGen)
}

// delegatingUI renders a child Element of its own and then delegates to a Template on the
// same Element, which Renderer.JawsRender explicitly permits.
type delegatingUI struct {
	tmpl     Template
	handle   bool          // capture the delegate's error instead of propagating it
	child    *jaws.Element // the Element this renderer created itself
	delegErr error         // the error the delegate returned
}

func (u *delegatingUI) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	u.child = elem.Request.NewElement(NewSpan(testHTMLGetter("mine")))
	if err = u.child.JawsRender(w, nil); err != nil {
		return
	}
	// Buffer the delegate's output so a handled failure discards its partial markup, and
	// emit a wrapper carrying the delegate's Jid so a later update has a real target.
	var sb strings.Builder
	u.delegErr = u.tmpl.JawsRender(elem, &sb, params)
	if u.delegErr != nil && u.handle {
		b := elem.Jid().AppendStartTagAttr(nil, "div")
		b = append(b, '>')
		if _, err = w.Write(append(b, "</div>"...)); err != nil {
			return
		}
		return nil
	}
	_, err = io.WriteString(w, sb.String())
	if err == nil {
		err = u.delegErr
	}
	return
}

func (u *delegatingUI) JawsUpdate(elem *jaws.Element) { u.tmpl.JawsUpdate(elem) }

// TestTemplate_DelegatedRenderKeepsTheDelegatorsChild checks that Template rollback is scoped
// to the Elements it created, leaving a child the delegating renderer made itself untouched.
// The delegate creates a nested Element before failing, so the assertion covers a non-empty
// generation.
func TestTemplate_DelegatedRenderKeepsTheDelegatorsChild(t *testing.T) {
	for _, tt := range []struct {
		name   string
		handle bool
		want   int // registered elements afterwards
	}{
		// wrapper + the delegator's own child; the delegate's nested Element is gone
		{"handled", true, 2},
		// NewUI additionally deletes the failed root, leaving only the delegator's child:
		// it was created outside the writer, so nothing owns it either way
		{"propagated", false, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, rq := newStateRequest(t)

			dot := &ownedDot{fail: errOwnedDotCheck}
			ui := &delegatingUI{tmpl: NewTemplate("div", "state-failafter", dot), handle: tt.handle}
			var sb strings.Builder
			rw := RequestWriter{Request: rq, Writer: &sb}
			err := rw.NewUI(ui)

			if !errors.Is(ui.delegErr, errOwnedDotCheck) {
				t.Fatalf("delegate error = %v, want %v", ui.delegErr, errOwnedDotCheck)
			}
			if tt.handle {
				if err != nil {
					t.Fatalf("handled render returned %v, want nil", err)
				}
			} else if !errors.Is(err, errOwnedDotCheck) {
				t.Fatalf("propagated render returned %v, want %v", err, errOwnedDotCheck)
			}
			// The delegator created its child outside the writer, so the Template's
			// generation never contained it and neither rollback path can reach it.
			if ui.child.Deleted() {
				t.Fatal("the Template's rollback deleted a child the delegator created itself")
			}
			if got := countRegistered(t, rq); got != tt.want {
				t.Fatalf("registered elements = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestTemplate_DelegatedRenderClaimSurvivesHandledError checks the documented consequence
// of claiming before any side effect: a handled render failure leaves the claim in place, so
// a later update still runs.
func TestTemplate_DelegatedRenderClaimSurvivesHandledError(t *testing.T) {
	logger := new(templateLogger)
	_, rq := newConfiguredStateRequest(t, withLogger(logger))

	dot := &ownedDot{fail: errOwnedDotCheck}
	ui := &delegatingUI{tmpl: NewTemplate("div", "state-failafter", dot), handle: true}
	var sb strings.Builder
	rw := RequestWriter{Request: rq, Writer: &sb}
	if err := rw.NewUI(ui); err != nil {
		t.Fatal(err)
	}

	// The claim persists, so this update executes rather than reporting an unclaimed slot.
	dot.setFail(nil)
	elem := rq.GetElementByJid(jaws.Jid(1))
	ui.JawsUpdate(elem)
	if len(logger.errors) != 0 {
		t.Fatalf("logged errors = %v, want none: the claim should have survived", logger.errors)
	}
}

// contendingUI delegates to two Templates on one Element, which the one-claimer rule
// rejects for the second.
type contendingUI struct {
	first, second Template
	secondErr     error
	handle        bool
}

func (u *contendingUI) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	if err = u.first.JawsRender(elem, w, nil); err != nil {
		return
	}
	var sb strings.Builder
	u.secondErr = u.second.JawsRender(elem, &sb, params)
	if sb.Len() > 0 {
		return errors.New("the rejected Template wrote output")
	}
	if u.handle {
		return nil
	}
	return u.secondErr
}

func (u *contendingUI) JawsUpdate(elem *jaws.Element) { u.first.JawsUpdate(elem) }

// TestTemplate_SecondClaimRegistersNoHandler covers the one side effect
// TestTemplate_SecondClaimOnOneElementFails cannot reach: handlers are unexported, so the
// only way to observe that the rejected Template registered none is to deliver an event
// and watch it not arrive.
//
// A bare "the handler was not called" assertion would pass even if events were broken
// entirely, so a control Element rendered by a Template that does claim carries the same
// handler through the same params path, and its second click doubles as the barrier
// proving the rejected Element's click was already processed.
func TestTemplate_SecondClaimRegistersNoHandler(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	if err = jw.AddTemplateLookuper(template.Must(template.New("state").Parse(templateStateTemplates))); err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	tr := jawstest.NewTestRequest(jw, nil)
	t.Cleanup(func() {
		tr.Close()
		<-tr.DoneCh
	})
	<-tr.ReadyCh

	var sb strings.Builder
	control := &clickCountingDot{}
	controlElem := tr.NewElement(NewTemplate("div", "state-plain", tag.Tag("control")))
	if err = controlElem.JawsRender(&sb, []any{control}); err != nil {
		t.Fatal(err)
	}

	// contendingUI hands params to the second Template only, so this handler is offered
	// exclusively to the Template whose claim is rejected.
	rejected := &clickCountingDot{}
	ui := &contendingUI{
		first:  NewTemplate("div", "state-plain", tag.Tag("first")),
		second: NewTemplate("div", "state-b", tag.Tag("second")),
		handle: true,
	}
	elem := tr.NewElement(ui)
	if err = elem.JawsRender(&sb, []any{rejected}); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(ui.secondErr, jaws.ErrElementStateClaimed) {
		t.Fatalf("second render = %v, want %v", ui.secondErr, jaws.ErrElementStateClaimed)
	}

	// Click data is "X Y kstate name"; a bare name does not parse.
	click := func(j jid.Jid) { tr.InCh <- wire.WsMsg{Jid: j, What: what.Click, Data: "1 2 0 btn"} }
	awaitClicks := func(want int32) {
		t.Helper()
		for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
			if control.clicks.Load() >= want {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatalf("control clicks = %d, want %d", control.clicks.Load(), want)
	}

	click(controlElem.Jid())
	awaitClicks(1) // the params handler path works at all

	click(elem.Jid())
	// InCh is unbuffered and the loop consumes it in order, so once this second control
	// click has been handled the preceding one must already have been.
	click(controlElem.Jid())
	awaitClicks(2)

	if got := rejected.clicks.Load(); got != 0 {
		t.Fatalf("clicks delivered to the rejected Template's handler = %d, want 0", got)
	}
}

// TestTemplate_SecondClaimOnOneElementFails covers the contention path through the
// supported shape — one renderer delegating to two Templates — and proves the claim
// precedes every side effect: the rejected Template writes nothing and registers no tag.
// TestTemplate_SecondClaimRegistersNoHandler covers the handler list.
func TestTemplate_SecondClaimOnOneElementFails(t *testing.T) {
	for _, tt := range []struct {
		name   string
		handle bool
	}{
		{"handled", true},
		{"propagated", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, rq := newStateRequest(t)

			firstDot := tag.Tag("first")
			secondDot := tag.Tag("second")
			ui := &contendingUI{
				first:  NewTemplate("div", "state-plain", firstDot),
				second: NewTemplate("div", "state-b", secondDot),
				handle: tt.handle,
			}
			var sb strings.Builder
			rw := RequestWriter{Request: rq, Writer: &sb}
			err := rw.NewUI(ui, tag.Tag("param"))

			if !errors.Is(ui.secondErr, jaws.ErrElementStateClaimed) {
				t.Fatalf("second render = %v, want %v", ui.secondErr, jaws.ErrElementStateClaimed)
			}
			if tt.handle {
				if err != nil {
					t.Fatalf("handled render returned %v, want nil", err)
				}
				if got := len(rq.GetElements(firstDot)); got != 1 {
					t.Fatalf("elements tagged by the first Template = %d, want 1", got)
				}
			} else if !errors.Is(err, jaws.ErrElementStateClaimed) {
				t.Fatalf("propagated render returned %v, want %v", err, jaws.ErrElementStateClaimed)
			}
			// The rejected Template must not have registered its Dot tag or the params tag.
			if got := len(rq.GetElements(secondDot)); got != 0 {
				t.Fatalf("elements tagged by the rejected Template = %d, want 0", got)
			}
			if got := len(rq.GetElements(tag.Tag("param"))); got != 0 {
				t.Fatalf("elements tagged from the rejected render's params = %d, want 0", got)
			}
		})
	}
}

// TestTemplate_UpdateToleratesTypedNilContainerState checks the cleanup walk against a
// slot holding a typed-nil container state. The exact state-type switch must check nil
// before calling its ownership method.
func TestTemplate_UpdateToleratesTypedNilContainerState(t *testing.T) {
	_, rq := newStateRequest(t)

	// The nested child is a Span rather than a nested Template, whose Element would have
	// claimed its own slot already.
	dot := &ownedDot{}
	tmpl := NewTemplate("div", "state-span", dot)
	elem := renderOwned(t, rq, tmpl)
	if got := countRegistered(t, rq); got != 2 {
		t.Fatalf("registered elements after render = %d, want 2", got)
	}

	child := rq.GetElementByJid(elem.Jid() + 1)
	if child == nil {
		t.Fatal("expected the nested span Element")
	}
	if err := jaws.SetElementState(child, (*containerState)(nil)); err != nil {
		t.Fatal(err)
	}

	tmpl.JawsUpdate(elem) // must not panic walking the typed-nil state
	if got := countRegistered(t, rq); got != 2 {
		t.Fatalf("registered elements after update = %d, want 2", got)
	}
}

// TestTemplate_ZeroValueUpdateStaysSilent preserves the zero value's update behavior: it
// has no wrapper target, so it returns before consulting the state slot.
func TestTemplate_ZeroValueUpdateStaysSilent(t *testing.T) {
	_, rq := newStateRequest(t)

	// No logger is configured, so MustLog would panic if this reported anything.
	var tmpl Template
	tmpl.JawsUpdate(rq.NewElement(tmpl))
}
