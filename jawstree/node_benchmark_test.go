package jawstree

import (
	"encoding/json"
	"math"
	"strings"
	"sync"
	"testing"
)

var (
	resolveIndexBenchSink  *Node
	applyDeltaBenchSink    bool
	jsonStringLenBenchSink int64
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

// BenchmarkJSONStringLen compares the non-allocating escaped-length counter New uses to
// weigh a node name against a json.Marshal-and-measure baseline, across a typical short
// label, a large label, and a hostile over-budget name. "counter" stays allocation-free
// and, given a budget, stops early; "marshal" always allocates and encodes the whole
// string before its length can be measured.
func BenchmarkJSONStringLen(b *testing.B) {
	cases := []struct {
		name  string
		s     string
		limit int64
	}{
		{"short", "a typical node label", math.MaxInt64},
		{"large", strings.Repeat("node label ", 1024), math.MaxInt64}, // ~11 KiB
		{"hostile", strings.Repeat("x", 4<<20), 64},                   // 4 MiB, tiny budget
	}
	for _, tc := range cases {
		b.Run(tc.name+"/counter", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				jsonStringLenBenchSink = jsonStringLen(tc.s, tc.limit)
			}
		})
		b.Run(tc.name+"/marshal", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				enc, _ := json.Marshal(tc.s)
				jsonStringLenBenchSink = int64(len(enc))
			}
		})
	}
}

// TestJSONStringLenZeroAllocs pins the escaped-length counter at zero allocations, the
// contract that lets New measure an untrusted, arbitrarily large name without allocating.
func TestJSONStringLenZeroAllocs(t *testing.T) {
	name := strings.Repeat("x", 4096)
	allocs := testing.AllocsPerRun(100, func() {
		jsonStringLenBenchSink = jsonStringLen(name, math.MaxInt64)
	})
	if allocs != 0 {
		t.Errorf("jsonStringLen allocated %g times, want 0", allocs)
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
