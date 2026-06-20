package named

import (
	"errors"
	"html/template"
	"io"
	"sync/atomic"
	"testing"
	"testing/synctest"
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
	if err := nb.JawsSet(e, true); !errors.Is(err, jaws.ErrValueUnchanged) {
		t.Fatalf("expected ErrValueUnchanged, got %v", err)
	}
}

func TestNamedBool_JawsSetCheckedValueDeselectsCheckedSibling(t *testing.T) {
	nba := NewBoolArray(false).Add("one", "one").Add("two", "two")
	one := nba.data[0]
	two := nba.data[1]
	one.Set(true)
	two.Set(true)

	_, rq := newCoreRequest(t)
	elem := rq.NewElement(noopUI{})
	if err := one.JawsSet(elem, true); err != nil {
		t.Fatalf("JawsSet returned %v, want nil because sibling state changed", err)
	}
	if !one.Checked() {
		t.Fatal("target should remain checked")
	}
	if two.Checked() {
		t.Fatal("single-select sibling should be deselected")
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

// waitForDirtyProbes advances the bubble's fake clock past the Serve loop's
// updateTicker (DefaultUpdateInterval, 100ms) so queued dirt is distributed and
// broadcast, lets the request process loop run the resulting JawsUpdate calls,
// then asserts the probes fired. It must be called from within a synctest bubble.
func waitForDirtyProbes(t *testing.T, done func() bool) {
	t.Helper()
	time.Sleep(200 * time.Millisecond)
	synctest.Wait()
	if !done() {
		t.Fatal("timed out waiting for dirty probes")
	}
}

func TestNamedBool_JawsSetDirtiesDeselectedSiblings(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, rq := newTestRequest(t)
		defer closeBubbleRequest(jw, rq)

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
	})
}

// TestNamedBoolArray_JawsSetDirtiesChangedBoolsAndArray asserts that selecting
// through the array dirties the newly-checked Bool, the deselected sibling, AND the
// array tag (mirroring Bool.JawsSet), so consumers binding individual Bools update,
// not only those binding the array.
func TestNamedBoolArray_JawsSetDirtiesChangedBoolsAndArray(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, rq := newTestRequest(t)
		defer closeBubbleRequest(jw, rq)

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
		if err := nba.JawsSet(trigger, "two"); !errors.Is(err, jaws.ErrValueUnchanged) {
			t.Fatalf("re-set error = %v, want ErrValueUnchanged", err)
		}
	})
}
