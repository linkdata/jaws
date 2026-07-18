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

// TestNew_NodeCapBoundary pins the exact node-count cutoff: a tree of exactly
// MaxTreeNodes nodes is accepted and one node more is rejected. The tree is a shallow
// fan (depth 1), so neither the depth nor the render-byte bound can fire and only the
// node cap governs.
func TestNew_NodeCapBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a MaxTreeNodes+1 tree")
	}
	// MaxTreeNodes children plus the root is MaxTreeNodes+1 nodes; the first
	// MaxTreeNodes-1 children plus the root is exactly MaxTreeNodes.
	children := make([]*Node, MaxTreeNodes)
	for i := range children {
		children[i] = &Node{Name: "n"}
	}
	var mu sync.Mutex
	if _, err := New(&mu, &Node{Children: children[:MaxTreeNodes-1]}); err != nil {
		t.Fatalf("exactly MaxTreeNodes nodes should be accepted, got %v", err)
	}
	if _, err := New(&mu, &Node{Children: children}); !errors.Is(err, ErrInvalidTree) {
		t.Fatalf("MaxTreeNodes+1 nodes err = %v, want ErrInvalidTree", err)
	}
}

// TestIndexEnforcesCapDuringTraversal pins that index rejects an oversized tree
// mid-traversal (returning ErrInvalidTree once byIndex reaches MaxTreeNodes) rather
// than after fully indexing it. Bounding byIndex bounds the recursion depth, so a
// pathologically deep single-child tree can no longer overflow the stack before the
// cap is applied; the deep shape shares this guard with the wide one exercised here.
func TestIndexEnforcesCapDuringTraversal(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a MaxTreeNodes+1 tree")
	}
	children := make([]*Node, MaxTreeNodes+1)
	for i := range children {
		children[i] = &Node{Name: "n"}
	}
	tr := &Tree{}
	err := tr.index(&Node{Children: children}, "", make(map[*Node]bool), new(int), 0)
	if !errors.Is(err, ErrInvalidTree) {
		t.Fatalf("err = %v, want ErrInvalidTree", err)
	}
	if len(tr.byIndex) > MaxTreeNodes {
		t.Fatalf("byIndex grew to %d, want <= %d", len(tr.byIndex), MaxTreeNodes)
	}
}

// buildChain returns the root of a single-child chain with the given number of
// descendants, so its deepest node sits at depth == descendants.
func buildChain(descendants int) *Node {
	root := &Node{Name: "root"}
	cur := root
	for i := 0; i < descendants; i++ {
		child := &Node{Name: "n"}
		cur.Children = []*Node{child}
		cur = child
	}
	return root
}

// TestNew_RejectsTreeDeeperThanMaxTreeDepth pins the depth bound that keeps a tree the
// browser cannot render from reaching the client: the vendored treeview.js recurses
// once per level, so an over-deep tree would overflow the browser stack. New accepts a
// chain exactly at MaxTreeDepth and rejects one a single level deeper with
// ErrInvalidTree. Such a chain stays far below MaxTreeNodes, and its depth-weighted ID
// bytes stay under MaxTreeRenderBytes, so only the depth bound can reject the deeper one.
func TestNew_RejectsTreeDeeperThanMaxTreeDepth(t *testing.T) {
	var mu sync.Mutex
	if _, err := New(&mu, buildChain(MaxTreeDepth)); err != nil {
		t.Fatalf("depth %d should be accepted, got %v", MaxTreeDepth, err)
	}
	if _, err := New(&mu, buildChain(MaxTreeDepth+1)); !errors.Is(err, ErrInvalidTree) {
		t.Fatalf("depth %d err = %v, want ErrInvalidTree", MaxTreeDepth+1, err)
	}
}

// TestNew_RejectsWideDeepTreeByRenderBytes is the combined depth-plus-width regression:
// a spine to the deepest allowed level with a wide fan-out of leaves there passes the
// node, depth, and raw ID-byte checks yet is rejected by ErrInvalidTree, because each
// leaf's long ID is retained once per ancestor and the depth-weighted total exceeds
// MaxTreeRenderBytes. This is the shape whose client retention the independent caps miss
// (a 47,505-leaf version retains ~8.5 billion ID chars in the browser at ~67 MB of
// server IDs); only MaxTreeRenderBytes catches it.
//
// The leaf count is derived from the cap and is small: each leaf contributes about
// MaxTreeDepth*(11*MaxTreeDepth) depth-weighted bytes, so a few hundred leaves overshoot
// 64 MiB. The tree is tiny (node count and depth both far inside their bounds), so those
// guards demonstrably cannot be what rejects it.
func TestNew_RejectsWideDeepTreeByRenderBytes(t *testing.T) {
	// Leaves sit at depth MaxTreeDepth, each with an ID of at least ~11*MaxTreeDepth
	// bytes; depth-weighted that is ~MaxTreeDepth*11*MaxTreeDepth per leaf. Double the
	// derived count to overshoot the cap robustly.
	leaves := 2 * (MaxTreeRenderBytes / (11 * MaxTreeDepth * MaxTreeDepth))
	root := buildChain(MaxTreeDepth - 1) // deepest spine node sits at depth MaxTreeDepth-1
	deepest := root
	for len(deepest.Children) > 0 {
		deepest = deepest.Children[0]
	}
	deepest.Children = make([]*Node, leaves)
	for i := range deepest.Children {
		deepest.Children[i] = &Node{Name: "leaf"}
	}
	if total := MaxTreeDepth + leaves; total >= MaxTreeNodes {
		t.Fatalf("test tree has %d nodes, not below MaxTreeNodes %d", total, MaxTreeNodes)
	}
	var mu sync.Mutex
	if _, err := New(&mu, root); !errors.Is(err, ErrInvalidTree) {
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
