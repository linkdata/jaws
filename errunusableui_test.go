package jaws

import (
	"context"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
)

// ifaceSliceUI is statically comparable (an interface field) but panics when compared
// at runtime, since the interface holds a slice.
type ifaceSliceUI struct{ v any }

func (ifaceSliceUI) JawsRender(*Element, io.Writer, []any) error { return nil }
func (ifaceSliceUI) JawsUpdate(*Element)                         {}

// typedNilUI has pointer-receiver methods that tolerate a nil receiver, so a typed nil
// (*typedNilUI)(nil) is a usable UI value that renders without dereferencing.
type typedNilUI struct{ s string }

func (u *typedNilUI) JawsRender(_ *Element, w io.Writer, _ []any) error {
	s := "typednil"
	if u != nil {
		s = u.s
	}
	_, err := io.WriteString(w, s)
	return err
}
func (*typedNilUI) JawsUpdate(*Element) {}

func TestNewErrUnusableUI(t *testing.T) {
	tests := []struct {
		name    string
		ui      UI
		wantErr bool
	}{
		{"nil", nil, true},
		{"nan struct", nonReflexiveUI{f: math.NaN()}, true},
		{"map field (statically incomparable)", testUnhashableUI{m: map[string]int{"x": 1}}, true},
		{"interface holding slice (runtime-incomparable)", ifaceSliceUI{v: []int{1}}, true},
		{"valid pointer", &testUi{}, false},
		{"valid struct", nonReflexiveUI{f: 1.5}, false},
		// A typed nil is comparable and equal to itself, so it is usable; only a nil
		// interface is rejected.
		{"typed nil pointer", (*typedNilUI)(nil), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewErrUnusableUI(tt.ui)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("NewErrUnusableUI = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("NewErrUnusableUI = nil, want error")
			}
			// The error stands in for both tag identities so callers can match either.
			if !errors.Is(err, tag.ErrNotUsableAsTag) {
				t.Errorf("err does not match tag.ErrNotUsableAsTag: %v", err)
			}
			if !errors.Is(err, tag.ErrNotComparable) {
				t.Errorf("err does not match tag.ErrNotComparable: %v", err)
			}
		})
	}
}

// TestNewElementNilUIRendersNoop verifies that NewElement(nil) does not terminate the
// Request — a nil UI is never reconciled by a container, so it is harmless — and
// returns an Element that renders and updates as a no-op rather than panicking on the
// nil UI. A nil child returned from a container is instead rejected; see
// TestContainerTerminatesOnUnusableChild.
func TestNewElementNilUIRendersNoop(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(nil)
	if cause := context.Cause(rq.Context()); cause != nil {
		t.Fatalf("NewElement(nil) cancelled the Request: %v", cause)
	}

	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatalf("JawsRender err = %v, want nil", err)
	}
	if sb.Len() != 0 {
		t.Fatalf("nil-UI render wrote %q, want empty", sb.String())
	}
	elem.JawsUpdate() // must not panic
}

// TestNewElementTypedNilUIDispatchesToRenderer documents that a typed nil UI (a
// non-nil interface holding a nil pointer) is usable — comparable and equal to itself
// — and dispatches to its Renderer rather than being treated as unusable. Tolerating
// the nil receiver is the concrete type's responsibility; typedNilUI does so.
func TestNewElementTypedNilUIDispatchesToRenderer(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	var ui UI = (*typedNilUI)(nil)
	if err := NewErrUnusableUI(ui); err != nil {
		t.Fatalf("NewErrUnusableUI(typed nil) = %v, want nil (usable)", err)
	}

	elem := rq.NewElement(ui)
	if cause := context.Cause(rq.Context()); cause != nil {
		t.Fatalf("NewElement(typed nil) cancelled the Request: %v", cause)
	}
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatalf("JawsRender err = %v", err)
	}
	if sb.String() != "typednil" {
		t.Fatalf("render = %q, want %q", sb.String(), "typednil")
	}
}
