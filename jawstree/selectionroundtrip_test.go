package jawstree_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/linkdata/jaws/jawstree"
)

// TestNode_SelectedRoundTrip locks the GetSelected/SetSelected round-trip on a
// distinct-name tree: applying a selection then reading it back yields the same
// name-path set, and re-applying that read-back changes nothing.
func TestNode_SelectedRoundTrip(t *testing.T) {
	// The name-path API (HasNames/GetNames) needs Parent back-pointers, which New
	// wires; wire them here directly since this builds a raw Node tree.
	build := func() *jawstree.Node {
		root := &jawstree.Node{Name: "root"}
		a := &jawstree.Node{Name: "a", Parent: root}
		a0 := &jawstree.Node{Name: "a0", Parent: a}
		a1 := &jawstree.Node{Name: "a1", Parent: a}
		b := &jawstree.Node{Name: "b", Parent: root}
		b0 := &jawstree.Node{Name: "b0", Parent: b}
		a.Children = []*jawstree.Node{a0, a1}
		b.Children = []*jawstree.Node{b0}
		root.Children = []*jawstree.Node{a, b}
		return root
	}
	asSet := func(nameLists [][]string) []string {
		out := make([]string, 0, len(nameLists))
		for _, names := range nameLists {
			out = append(out, strings.Join(names, "/"))
		}
		slices.Sort(out)
		return out
	}
	tests := [][][]string{
		nil,
		{{"a"}},
		{{"a", "a0"}, {"b"}},
		{{"a"}, {"a", "a0"}, {"a", "a1"}, {"b"}, {"b", "b0"}},
	}
	for _, want := range tests {
		root := build()
		root.SetSelected(want)
		got := root.GetSelected()
		if !slices.Equal(asSet(got), asSet(want)) {
			t.Errorf("SetSelected(%v) -> GetSelected() = %v, want %v", want, got, want)
		}
		if changed := root.SetSelected(got); len(changed) != 0 {
			t.Errorf("re-applying GetSelected result changed %d nodes, want 0", len(changed))
		}
	}
}
