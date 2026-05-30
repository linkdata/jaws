package named

import (
	"html/template"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
)

func TestNamedBool(t *testing.T) {
	nba := NewBoolArray(false)
	nba.Add("1", "one")
	nb := nba.data[0]

	_, rq := newCoreRequest(t)
	e := rq.NewElement(noopUI{})

	if nb.Array() != nba {
		t.Fatalf("array mismatch: got %p want %p", nb.Array(), nba)
	}
	if nb.Name() != "1" {
		t.Fatalf("name mismatch: got %q want %q", nb.Name(), "1")
	}
	if nb.HTML() != template.HTML("one") {
		t.Fatalf("html mismatch: got %q want %q", nb.HTML(), template.HTML("one"))
	}

	if got := nb.JawsGetHTML(nil); got != nb.HTML() {
		t.Fatalf("JawsGetHTML mismatch: got %q want %q", got, nb.HTML())
	}

	if err := nb.JawsSet(e, true); err != nil {
		t.Fatal(err)
	}
	if !nb.Checked() {
		t.Fatal("expected checked true")
	}
	if got := nb.JawsGet(nil); got != nb.Checked() {
		t.Fatalf("JawsGet mismatch: got %v want %v", got, nb.Checked())
	}
	if err := nb.JawsSet(e, true); err != jaws.ErrValueUnchanged {
		t.Fatalf("expected ErrValueUnchanged, got %v", err)
	}
}

type dirtyProbe struct {
	hits *atomic.Int32
}

func (probe *dirtyProbe) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return nil
}

func (probe *dirtyProbe) JawsUpdate(elem *jaws.Element) {
	probe.hits.Add(1)
}

func registerDirtyProbe(rq *jawstest.TestRequest, tag any, hits *atomic.Int32) {
	elem := rq.NewElement(&dirtyProbe{hits: hits})
	elem.Tag(tag)
}

func waitForDirtyProbes(t *testing.T, done func() bool) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for !done() {
		select {
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		case <-timer.C:
			t.Fatal("timed out waiting for dirty probes")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestNamedBool_JawsSetDirtiesDeselectedSiblings(t *testing.T) {
	_, rq := newTestRequest(t)

	nba := NewBoolArray(false).Add("one", "one").Add("two", "two")
	one := nba.data[0]
	two := nba.data[1]
	nba.Set("one", true)

	var oneHits, twoHits, groupHits atomic.Int32
	registerDirtyProbe(rq, one, &oneHits)
	registerDirtyProbe(rq, two, &twoHits)
	registerDirtyProbe(rq, nba, &groupHits)

	trigger := rq.NewElement(noopUI{})
	if err := two.JawsSet(trigger, true); err != nil {
		t.Fatal(err)
	}
	if one.Checked() {
		t.Fatal("expected first bool to be deselected")
	}
	if !two.Checked() {
		t.Fatal("expected second bool to be selected")
	}

	waitForDirtyProbes(t, func() bool {
		return oneHits.Load() > 0 && twoHits.Load() > 0 && groupHits.Load() > 0
	})
}

// TestNamedBoolArray_JawsSetDirtiesChangedBoolsAndArray asserts that selecting
// through the array dirties the newly-checked Bool, the deselected sibling, AND
// the array tag (mirroring Bool.JawsSet), not just the array tag. Before the fix
// only the array was dirtied, so consumers binding individual Bools never updated.
func TestNamedBoolArray_JawsSetDirtiesChangedBoolsAndArray(t *testing.T) {
	_, rq := newTestRequest(t)

	nba := NewBoolArray(false).Add("one", "one").Add("two", "two")
	one := nba.data[0]
	two := nba.data[1]
	nba.Set("one", true) // start with "one" selected

	var oneHits, twoHits, groupHits atomic.Int32
	registerDirtyProbe(rq, one, &oneHits)
	registerDirtyProbe(rq, two, &twoHits)
	registerDirtyProbe(rq, nba, &groupHits)

	trigger := rq.NewElement(noopUI{})
	if err := nba.JawsSet(trigger, "two"); err != nil {
		t.Fatal(err)
	}
	if one.Checked() {
		t.Fatal("expected first bool to be deselected")
	}
	if !two.Checked() {
		t.Fatal("expected second bool to be selected")
	}

	waitForDirtyProbes(t, func() bool {
		return oneHits.Load() > 0 && twoHits.Load() > 0 && groupHits.Load() > 0
	})

	// Setting the same selection again is a no-op and must report ErrValueUnchanged.
	if err := nba.JawsSet(trigger, "two"); err != jaws.ErrValueUnchanged {
		t.Fatalf("re-set error = %v, want ErrValueUnchanged", err)
	}
}
