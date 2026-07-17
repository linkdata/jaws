package jawstree

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestNew_Validation(t *testing.T) {
	var mu sync.Mutex
	cases := []struct {
		name    string
		root    *Node
		options []Option
		wantErr error
	}{
		{"nil root", nil, nil, ErrInvalidTree},
		{"negative option", &Node{}, []Option{Option(-1)}, ErrInvalidTree},
		{"unknown option bit", &Node{}, []Option{Option(1 << 20)}, ErrInvalidTree},
		{"root selected", &Node{Selected: true, Children: []*Node{{Name: "a"}}}, nil, ErrInvalidSelection},
		{"disabled selected", &Node{Children: []*Node{{Name: "a", Selected: true, Disabled: true}}}, nil, ErrInvalidSelection},
		{"single-select two selected", &Node{Children: []*Node{{Name: "a", Selected: true}, {Name: "b", Selected: true}}}, nil, ErrInvalidSelection},
		{"multi-select two selected ok", &Node{Children: []*Node{{Name: "a", Selected: true}, {Name: "b", Selected: true}}}, []Option{MultiSelectEnabled}, nil},
		{"node-selection-disabled with initial selection", &Node{Children: []*Node{{Name: "a", Selected: true}}}, []Option{NodeSelectionDisabled}, ErrInvalidSelection},
		{"node-selection-disabled empty ok", &Node{Children: []*Node{{Name: "a"}}}, []Option{NodeSelectionDisabled}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(&mu, tc.root, tc.options...)
			if tc.wantErr == nil {
				maybeError(t, err)
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestNew_RejectsCyclicAndSharedGraphs(t *testing.T) {
	var mu sync.Mutex
	t.Run("shared node", func(t *testing.T) {
		shared := &Node{Name: "shared"}
		if _, err := New(&mu, &Node{Children: []*Node{shared, shared}}); !errors.Is(err, ErrInvalidTree) {
			t.Fatalf("err = %v, want ErrInvalidTree", err)
		}
	})
	t.Run("cycle", func(t *testing.T) {
		a := &Node{Name: "a"}
		a.Children = []*Node{a}
		if _, err := New(&mu, &Node{Children: []*Node{a}}); !errors.Is(err, ErrInvalidTree) {
			t.Fatalf("err = %v, want ErrInvalidTree", err)
		}
	})
}

func TestNew_RejectsOverCap(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a MaxTreeNodes+1 tree")
	}
	children := make([]*Node, MaxTreeNodes+1)
	for i := range children {
		children[i] = &Node{Name: "n"}
	}
	var mu sync.Mutex
	if _, err := New(&mu, &Node{Children: children}); !errors.Is(err, ErrInvalidTree) {
		t.Fatalf("err = %v, want ErrInvalidTree", err)
	}
}

func TestApplyClientDelta_Gate(t *testing.T) {
	var mu sync.Mutex
	// preorder indices: root=0, a=1, b=2, b.c=3 (disabled)
	tree := mustNew(t, &mu, &Node{Children: []*Node{
		{Name: "a"},
		{Name: "b", Children: []*Node{{Name: "c", Disabled: true}}},
	}}, MultiSelectEnabled)

	for _, idx := range []int{0, -1, 99} {
		tree.Lock()
		_, err := tree.applyClientDelta([]int{idx}, nil)
		tree.Unlock()
		if !errors.Is(err, ErrPathRejected) {
			t.Fatalf("index %d: err = %v, want ErrPathRejected", idx, err)
		}
	}
	tree.Lock()
	_, err := tree.applyClientDelta([]int{3}, nil)
	tree.Unlock()
	if !errors.Is(err, ErrPathRejected) {
		t.Fatalf("disabled: err = %v, want ErrPathRejected", err)
	}

	// Merge: adding a then b leaves both selected.
	tree.Lock()
	_, _ = tree.applyClientDelta([]int{1}, nil)
	_, _ = tree.applyClientDelta([]int{2}, nil)
	tree.Unlock()
	if got := tree.selectedIndexes(); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("merged selection = %v, want [1 2]", got)
	}

	// A remove of an unselected node is a no-op.
	tree.Lock()
	changed, err := tree.applyClientDelta(nil, []int{3})
	tree.Unlock()
	maybeError(t, err)
	if changed {
		t.Fatal("removing an unselected node reported a change")
	}
}

func TestNodeSelectionDisabledRejectsSelection(t *testing.T) {
	var mu sync.Mutex
	tree := mustNew(t, &mu, &Node{Children: []*Node{{Name: "a"}}}, NodeSelectionDisabled)
	// The shared policy rejects selection through SetSelected and browser input alike.
	if err := tree.SetSelected([][]string{{"a"}}); !errors.Is(err, ErrInvalidSelection) {
		t.Fatalf("SetSelected err = %v, want ErrInvalidSelection", err)
	}
	tree.Lock()
	_, err := tree.applyClientDelta([]int{1}, nil)
	tree.Unlock()
	if !errors.Is(err, ErrInvalidSelection) {
		t.Fatalf("applyClientDelta err = %v, want ErrInvalidSelection", err)
	}
}

func TestApplyClientAbsolute_SingleSelectRejectsMultiple(t *testing.T) {
	var mu sync.Mutex
	tree := mustNew(t, &mu, &Node{Children: []*Node{{Name: "a"}, {Name: "b"}}})
	tree.Lock()
	_, err := tree.applyClientAbsolute([]int{1, 2})
	tree.Unlock()
	if !errors.Is(err, ErrPathRejected) {
		t.Fatalf("err = %v, want ErrPathRejected", err)
	}
}

func TestDecodeSelectionBitmap(t *testing.T) {
	// n=5, bits 1 and 3 set -> byte 0x0A -> base64 "Cg==".
	idx, err := decodeSelectionBitmap("Cg==", 5)
	maybeError(t, err)
	if !reflect.DeepEqual(idx, []int{1, 3}) {
		t.Fatalf("decoded = %v, want [1 3]", idx)
	}
	if _, err := decodeSelectionBitmap("Cg==", 100); !errors.Is(err, ErrPathRejected) {
		t.Fatalf("wrong-length err = %v, want ErrPathRejected", err)
	}
	if _, err := decodeSelectionBitmap("!!!", 5); !errors.Is(err, ErrPathRejected) {
		t.Fatalf("bad-base64 err = %v, want ErrPathRejected", err)
	}
}

func decodeSelectionPayload(t *testing.T, payload string, n int) []int {
	t.Helper()
	var p struct {
		S []int  `json:"s"`
		B string `json:"b"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		t.Fatalf("payload %q: %v", payload, err)
	}
	if p.B != "" {
		idx, err := decodeSelectionBitmap(p.B, n)
		maybeError(t, err)
		return idx
	}
	if p.S == nil {
		return []int{}
	}
	return p.S
}

func TestSelectionPayloadRoundTrip(t *testing.T) {
	var mu sync.Mutex
	children := make([]*Node, 20)
	for i := range children {
		children[i] = &Node{Name: "n"}
	}
	tree := mustNew(t, &mu, &Node{Children: children}, MultiSelectEnabled)
	n := len(tree.byIndex)

	for _, sel := range [][]int{{}, {1, 5, 20}, {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}} {
		tree.Lock()
		_, err := tree.applyClientAbsolute(sel)
		tree.Unlock()
		maybeError(t, err)
		tree.RLock()
		payload := tree.selectionPayloadLocked("Jid.1")
		tree.RUnlock()
		want := sel
		if want == nil {
			want = []int{}
		}
		if got := decodeSelectionPayload(t, payload, n); !reflect.DeepEqual(got, want) {
			t.Fatalf("round-trip for %v = %v (payload %q)", sel, got, payload)
		}
	}
}

// TestSelectionFrameFitsInboundLimit pins the [MaxTreeNodes] derivation: the largest
// possible selection frame — a full one-bit-per-node bitmap, base64-encoded, wrapped
// in the payload and Input framing — must stay within the jaws inbound WebSocket
// limit.
func TestSelectionFrameFitsInboundLimit(t *testing.T) {
	b64 := base64.StdEncoding.EncodedLen((MaxTreeNodes + 7) / 8)
	// Generous framing upper bound: "Input\t" + a long Jid + "\t" + the JSON wrapper
	// (key + base64) + "\n".
	frame := len("Input\t") + 32 + len("\t") + len(`{"key":"","b":""}`) + 32 + b64 + len("\n")
	if frame >= wsInboundLimit {
		t.Fatalf("worst-case selection frame %d bytes >= inbound limit %d; lower MaxTreeNodes", frame, wsInboundLimit)
	}
}
