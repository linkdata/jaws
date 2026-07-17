package jawstree_test

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/linkdata/jaws/jawstree"
)

// TestTree_SelectedRoundTrip locks the GetSelected/SetSelected round-trip on a
// distinct-name multi-select tree: applying a selection then reading it back yields
// the same name-path set, and re-applying that read-back changes nothing.
func TestTree_SelectedRoundTrip(t *testing.T) {
	// New wires the Parent back-pointers the name-path API needs.
	build := func() *jawstree.Node {
		a := &jawstree.Node{Name: "a", Children: []*jawstree.Node{{Name: "a0"}, {Name: "a1"}}}
		b := &jawstree.Node{Name: "b", Children: []*jawstree.Node{{Name: "b0"}}}
		return &jawstree.Node{Name: "root", Children: []*jawstree.Node{a, b}}
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
		var mu sync.RWMutex
		tree, err := jawstree.New(&mu, build(), jawstree.MultiSelectEnabled)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if err := tree.SetSelected(want); err != nil {
			t.Fatalf("SetSelected(%v): %v", want, err)
		}
		got := tree.GetSelected()
		if !slices.Equal(asSet(got), asSet(want)) {
			t.Errorf("SetSelected(%v) -> GetSelected() = %v, want %v", want, got, want)
		}
		if err := tree.SetSelected(got); err != nil {
			t.Fatalf("re-applying GetSelected result: %v", err)
		}
		if got2 := tree.GetSelected(); !slices.Equal(asSet(got2), asSet(want)) {
			t.Errorf("re-applying GetSelected result changed selection to %v", got2)
		}
	}
}
