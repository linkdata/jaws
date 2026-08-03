package ui

import (
	"errors"
	"html/template"
	"io"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

const templateStateTemplates = `
{{define "state-parent"}}{{$.RequestWriter.Template "" "state-leaf" $.Dot}}{{end}}
{{define "state-leaf"}}leaf{{end}}
{{define "state-failafter"}}{{$.RequestWriter.Template "" "state-leaf" $.Dot}}{{$.Dot.Check}}{{end}}
{{define "state-plain"}}plain{{end}}
{{define "state-b"}}b{{end}}
{{define "state-span"}}{{$.RequestWriter.Span "x"}}{{end}}
`

func newStateRequest(t *testing.T) (*jaws.Jaws, *jaws.Request) {
	t.Helper()
	jw, rq := newCoreRequest(t)
	if err := jw.AddTemplateLookuper(template.Must(template.New("state").Parse(templateStateTemplates))); err != nil {
		t.Fatal(err)
	}
	return jw, rq
}

// TestTemplate_EqualValuesKeepIndependentGenerations covers what the state slot buys: one
// Template value may back several live Elements, each with its own generation, which was
// impossible while the ownership set lived on the widget.
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

	// Updating one must reclaim only its own generation.
	tmpl.JawsUpdate(first)
	if got := countRegistered(t, rq); got != 4 {
		t.Fatalf("registered elements after updating one = %d, want 4", got)
	}
	tmpl.JawsUpdate(second)
	if got := countRegistered(t, rq); got != 4 {
		t.Fatalf("registered elements after updating both = %d, want 4", got)
	}
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

// TestTemplate_DelegatedRenderKeepsTheDelegatorsChild is the case that decided this design:
// the Template's rollback is scoped to the Elements it created, so a child the delegating
// renderer made itself is untouched. The delegate creates a nested Element before failing,
// so the assertion is about a non-empty generation.
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
	jw, rq := newStateRequest(t)
	logger := new(templateLogger)
	jw.Logger = logger

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

// TestTemplate_SecondClaimOnOneElementFails covers the contention path through the
// supported shape — one renderer delegating to two Templates — and proves the claim
// precedes every side effect: the rejected Template writes nothing, registers no tag and
// adds no handler.
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

// TestTemplate_UpdateToleratesTypedNilElementState checks the cleanup walk against a state
// slot holding a typed nil that satisfies elementOwner through a promoted method. Matching
// the slot to *templateState rather than to elementOwner is what keeps this from
// dereferencing a nil receiver.
func TestTemplate_UpdateToleratesTypedNilElementState(t *testing.T) {
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
	if err := jaws.SetElementState(child, (*Container)(nil)); err != nil {
		t.Fatal(err)
	}

	tmpl.JawsUpdate(elem) // must not panic walking the typed-nil state
	if got := countRegistered(t, rq); got != 2 {
		t.Fatalf("registered elements after update = %d, want 2", got)
	}
}

// TestTemplate_UnwrappedUpdateStaysSilent pins the path the missing-claim diagnostic does
// not apply to: an unwrapped Template returns before the state slot is consulted.
func TestTemplate_UnwrappedUpdateStaysSilent(t *testing.T) {
	_, rq := newStateRequest(t)

	// No logger is configured, so MustLog would panic if this reported anything.
	tmpl := NewTemplate("", "state-plain", tag.Tag("dot"))
	tmpl.JawsUpdate(rq.NewElement(tmpl))
}
