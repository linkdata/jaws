package ui

import (
	"testing"

	"github.com/linkdata/jaws"
)

// TestContainerReconcileClearsVacatedContentSlots verifies that shrinking a
// container releases every Element reference beyond the new logical length. A
// removed nested Container is included because retaining that Element can also
// retain the application objects and ownership state reachable through it.
func TestContainerReconcileClearsVacatedContentSlots(t *testing.T) {
	_, rq := newCoreRequest(t)
	keep := NewSpan(testHTMLGetter("keep"))
	drop := NewSpan(testHTMLGetter("drop"))
	nestedProvider := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("nested leaf"))}}
	nested := NewContainer("section", nestedProvider)
	provider := &testContainer{contents: []jaws.UI{keep, drop, nested}}
	outer := NewContainer("div", provider)
	outerElem, _ := renderUI(t, rq, outer)

	initial := containerElements(t, outerElem)
	if len(initial) != 3 {
		t.Fatalf("initial children = %d, want 3", len(initial))
	}
	keepElem := initial[0]
	nestedElem := initial[2]
	nestedChildren := containerElements(t, nestedElem)
	if len(nestedChildren) != 1 {
		t.Fatalf("nested children = %d, want 1", len(nestedChildren))
	}
	nestedJid := nestedElem.Jid()
	nestedChildJid := nestedChildren[0].Jid()

	state := requireContainerState(t, outerElem)
	state.mu.Lock()
	backing := make([]*jaws.Element, len(state.contents), len(state.contents)+2)
	copy(backing, state.contents)
	state.contents = backing
	state.mu.Unlock()

	provider.contents = []jaws.UI{keep}
	outerElem.JawsUpdate()

	state.mu.Lock()
	gotLen := len(state.contents)
	gotBacking := append([]*jaws.Element(nil), state.contents[:cap(state.contents)]...)
	state.mu.Unlock()
	if gotLen != 1 {
		t.Fatalf("children after shrink = %d, want 1", gotLen)
	}
	if gotBacking[0] != keepElem {
		t.Fatalf("retained child = %p, want Element %v (%p)", gotBacking[0], keepElem.Jid(), keepElem)
	}
	for i, childElem := range gotBacking[gotLen:] {
		if childElem != nil {
			t.Errorf("vacated backing slot %d retains Element %v", gotLen+i, childElem.Jid())
		}
	}
	if got := rq.GetElementByJid(nestedJid); got != nil {
		t.Errorf("removed nested Element %v remains registered", nestedJid)
	}
	if got := rq.GetElementByJid(nestedChildJid); got != nil {
		t.Errorf("removed nested child Element %v remains registered", nestedChildJid)
	}
}
