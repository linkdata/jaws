package jaws

import (
	"errors"
	"sync"
	"testing"
)

type testElementState struct{ name string }

func TestElementState_ClaimOnce(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(&testUi{})
	if got := ElementState(elem); got != nil {
		t.Fatalf("unclaimed slot = %v, want nil", got)
	}

	first := &testElementState{name: "first"}
	if err := SetElementState(elem, first); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if got := ElementState(elem); got != first {
		t.Fatalf("loaded %v, want the claimed state", got)
	}

	// A second claim fails and leaves the original in place, including one carrying the
	// same dynamic type: same type does not mean same owner.
	for _, state := range []any{&testElementState{name: "same type"}, "other type"} {
		if err := SetElementState(elem, state); !errors.Is(err, ErrElementStateClaimed) {
			t.Fatalf("second claim with %T = %v, want %v", state, err, ErrElementStateClaimed)
		}
	}
	if got := ElementState(elem); got != first {
		t.Fatalf("state after rejected claims = %v, want the original", got)
	}

	// Slots are per Element.
	other := rq.NewElement(&testUi{})
	if got := ElementState(other); got != nil {
		t.Fatalf("second element's slot = %v, want nil", got)
	}
	if err := SetElementState(other, &testElementState{name: "second"}); err != nil {
		t.Fatalf("claiming a different element: %v", err)
	}
	if ElementState(elem) != first {
		t.Error("claiming another element disturbed the first element's state")
	}
}

func TestElementState_NilHandling(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(&testUi{})

	// A nil interface cannot be stored: it is indistinguishable from an unclaimed slot,
	// so accepting it would report success while leaving the slot claimable.
	if err := SetElementState(elem, nil); !errors.Is(err, ErrElementStateNil) {
		t.Fatalf("nil claim = %v, want %v", err, ErrElementStateNil)
	}
	if got := ElementState(elem); got != nil {
		t.Fatalf("slot after nil claim = %v, want still nil", got)
	}

	// A typed nil is a non-nil interface, so it does claim the slot.
	var typedNil *testElementState
	if err := SetElementState(elem, typedNil); err != nil {
		t.Fatalf("typed-nil claim: %v", err)
	}
	if err := SetElementState(elem, &testElementState{}); !errors.Is(err, ErrElementStateClaimed) {
		t.Fatalf("claim after typed nil = %v, want %v", err, ErrElementStateClaimed)
	}

	// The nil-argument check precedes the occupancy check, so a nil state against an
	// occupied slot still reports ErrElementStateNil.
	if err := SetElementState(elem, nil); !errors.Is(err, ErrElementStateNil) {
		t.Fatalf("nil claim on occupied slot = %v, want %v", err, ErrElementStateNil)
	}
}

func TestElementState_ConcurrentClaims(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(&testUi{})
	const claimants = 8

	var wg sync.WaitGroup
	errs := make([]error, claimants)
	states := make([]any, claimants)
	start := make(chan struct{})
	for i := range claimants {
		states[i] = &testElementState{name: "claimant"}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = SetElementState(elem, states[i])
		}()
	}
	close(start)
	wg.Wait()

	var winners int
	for i, err := range errs {
		switch {
		case err == nil:
			winners++
			if got := ElementState(elem); got != states[i] {
				t.Errorf("claimant %d succeeded but the slot holds %v", i, got)
			}
		case !errors.Is(err, ErrElementStateClaimed):
			t.Errorf("claimant %d = %v, want %v", i, err, ErrElementStateClaimed)
		}
	}
	if winners != 1 {
		t.Errorf("successful claims = %d, want exactly 1", winners)
	}
}
