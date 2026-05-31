package jawstree_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstree"
)

func TestNode_MarshalJSON(t *testing.T) {
	rootnode := &jawstree.Node{
		Name:     "foo",
		ID:       "bar",
		Selected: true,
		Children: []*jawstree.Node{
			{
				Name:     "child1",
				ID:       "",
				Disabled: true,
			},
			{
				Name: "child2",
			},
		},
	}
	b, _ := rootnode.MarshalJSON()
	want := `{"name":"foo","id":"bar","selected":true,"children":[{"name":"child1","selectable":false,"children":[]},{"name":"child2","children":[]}]}`
	if string(b) != want {
		t.Errorf("\n got %s\nwant %s\n", string(b), want)
	}
}

// TestNode_MarshalJSON_AdversarialNames ensures node names that are not clean
// ASCII (invalid UTF-8 from a raw filesystem entry, embedded quotes/newlines)
// still produce valid JSON. strconv.AppendQuote emitted Go-only \xNN escapes for
// invalid UTF-8, which broke JSON.parse on the client and stopped the tree from
// updating; appendJSONString must round-trip cleanly instead.
func TestNode_MarshalJSON_AdversarialNames(t *testing.T) {
	for _, name := range []string{
		string([]byte{0xff, 0xfe, 0x41}), // invalid UTF-8 bytes
		"weird\"name\nwith\tcontrol",     // quote, newline, tab
		"a<b>&",                          // HTML-ish and a JS line separator
	} {
		rootnode := &jawstree.Node{
			Name:     name,
			Children: []*jawstree.Node{{Name: name, ID: name}},
		}
		b, err := rootnode.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON(%q): %v", name, err)
		}
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Errorf("output for name %q is not valid JSON: %v\n got: %s", name, err, b)
		}
	}
}

// TestNode_JawsSetPath_Gate verifies the server-side path-set gate: only the
// per-node ".selected" bool is client-writable; any other path or a non-bool
// value is rejected without mutating the tree.
func TestNode_JawsSetPath_Gate(t *testing.T) {
	newTree := func() *jawstree.Node {
		return &jawstree.Node{Name: "root", Children: []*jawstree.Node{{Name: "child"}}}
	}

	t.Run("rejects non-selected path", func(t *testing.T) {
		root := newTree()
		if err := root.JawsSetPath(nil, "children.0.name", "renamed"); err == nil {
			t.Error("expected an error for a non-.selected path")
		}
		if got := root.Children[0].Name; got != "child" {
			t.Errorf("name was mutated to %q", got)
		}
	})

	t.Run("rejects non-bool value", func(t *testing.T) {
		root := newTree()
		if err := root.JawsSetPath(nil, "children.0.selected", "true"); err == nil {
			t.Error("expected an error for a non-bool .selected value")
		}
		if root.Children[0].Selected {
			t.Error("selected was mutated by a non-bool value")
		}
	})

	t.Run("sets selected bool", func(t *testing.T) {
		root := newTree()
		if err := root.JawsSetPath(nil, "children.0.selected", true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !root.Children[0].Selected {
			t.Error("selected was not set")
		}
		// Setting the same value again reports it as unchanged.
		if err := root.JawsSetPath(nil, "children.0.selected", true); !errors.Is(err, jaws.ErrValueUnchanged) {
			t.Errorf("expected ErrValueUnchanged, got %v", err)
		}
	})

	// Regression for the slice-growth bypass: a client Set with an out-of-range
	// child index (index == len) must be rejected without growing Children with a
	// nil node (which would crash every subsequent marshalJSON of a shared tree).
	t.Run("rejects out-of-range child index without growing the slice", func(t *testing.T) {
		for _, path := range []string{
			"children.1.selected",            // index == len(Children)
			"children.99.selected",           // far out of range
			"children.-1.selected",           // negative
			"children.x.selected",            // non-numeric
			"children.0.children.0.selected", // child has no children
		} {
			root := newTree()
			before := len(root.Children)
			if err := root.JawsSetPath(nil, path, true); err == nil {
				t.Errorf("path %q: expected an error, got nil", path)
			}
			if got := len(root.Children); got != before {
				t.Errorf("path %q: Children grew from %d to %d", path, before, got)
			}
			for i, c := range root.Children {
				if c == nil {
					t.Fatalf("path %q: Children[%d] is nil after rejected set", path, i)
				}
			}
			// Rendering the tree after a rejected set must not panic.
			if _, err := root.MarshalJSON(); err != nil {
				t.Errorf("path %q: MarshalJSON after rejected set: %v", path, err)
			}
		}
	})
}
