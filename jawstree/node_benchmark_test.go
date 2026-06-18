package jawstree

import "testing"

var resolveChildPathBenchSink *Node

// BenchmarkResolveChildPath guards the canonical child-path resolver against
// regression across shallow, deep and rejected (trailing-dot) paths. The resolver
// runs on every inbound tree-selection event, so it must stay allocation-free on
// the accepted paths.
func BenchmarkResolveChildPath(b *testing.B) {
	// Build a 3-deep tree so the deep path resolves.
	leaf := func(n string) *Node { return &Node{Name: n} }
	root := &Node{Name: "root", Children: []*Node{
		{Name: "a", Children: []*Node{
			leaf("a0"),
			{Name: "a1", Children: []*Node{leaf("a1.0"), leaf("a1.1"), leaf("a1.2")}},
		}},
		leaf("b"),
	}}

	cases := []struct {
		name string
		path string
	}{
		{"shallow", "children.1"},
		{"deep", "children.0.children.1.children.2"},
		{"rejected", "children.0.children.1.children.2."}, // trailing dot
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				n, _ := root.resolveChildPath(c.path)
				resolveChildPathBenchSink = n
			}
		})
	}
}
