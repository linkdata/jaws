package jawstree

import (
	"sync"
	"testing"
)

var (
	resolveIndexBenchSink *Node
	applyDeltaBenchSink   bool
)

func benchTree(b *testing.B) *Tree {
	b.Helper()
	// A 3-deep tree so shallow and deep indices both resolve.
	leaf := func(n string) *Node { return &Node{Name: n} }
	root := &Node{Name: "root", Children: []*Node{
		{Name: "a", Children: []*Node{
			leaf("a0"),
			{Name: "a1", Children: []*Node{leaf("a1.0"), leaf("a1.1"), leaf("a1.2")}},
		}},
		leaf("b"),
	}}
	var mu sync.Mutex
	tree, err := New(&mu, root, MultiSelectEnabled)
	if err != nil {
		b.Fatal(err)
	}
	return tree
}

// BenchmarkResolveIndex guards the O(1) wire-index resolver that runs on every
// inbound selection event; it must stay allocation-free.
func BenchmarkResolveIndex(b *testing.B) {
	tree := benchTree(b)
	last := len(tree.byIndex) - 1
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolveIndexBenchSink, _ = tree.resolveIndex(last)
	}
}

// TestResolveIndexZeroAllocs pins the resolver at zero allocations.
func TestResolveIndexZeroAllocs(t *testing.T) {
	var mu sync.Mutex
	tree, err := New(&mu, &Node{Children: []*Node{{Name: "a"}, {Name: "b"}}})
	maybeError(t, err)
	allocs := testing.AllocsPerRun(100, func() {
		resolveIndexBenchSink, _ = tree.resolveIndex(2)
	})
	if allocs != 0 {
		t.Errorf("resolveIndex allocated %g times, want 0", allocs)
	}
}

// BenchmarkApplyClientDelta measures the inbound merge path that runs under the
// write lock for each browser selection change.
func BenchmarkApplyClientDelta(b *testing.B) {
	tree := benchTree(b)
	last := len(tree.byIndex) - 1
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Lock()
		if i%2 == 0 {
			applyDeltaBenchSink, _ = tree.applyClientDelta([]int{last}, nil)
		} else {
			applyDeltaBenchSink, _ = tree.applyClientDelta(nil, []int{last})
		}
		tree.Unlock()
	}
}
