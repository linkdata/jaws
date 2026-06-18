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
// still produce valid JSON that the browser's JSON.parse accepts: appendJSONString
// must emit JSON-compatible escapes, never Go-only forms such as \xNN.
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
		if err := root.JawsSetPath(nil, "children.0.name", "renamed"); !errors.Is(err, jawstree.ErrPathRejected) {
			t.Errorf("expected ErrPathRejected for a non-.selected path, got %v", err)
		}
		if got := root.Children[0].Name; got != "child" {
			t.Errorf("name was mutated to %q", got)
		}
	})

	t.Run("rejects non-bool value", func(t *testing.T) {
		root := newTree()
		if err := root.JawsSetPath(nil, "children.0.selected", "true"); !errors.Is(err, jawstree.ErrPathRejected) {
			t.Errorf("expected ErrPathRejected for a non-bool .selected value, got %v", err)
		}
		if root.Children[0].Selected {
			t.Error("selected was mutated by a non-bool value")
		}
	})

	// The bare ".selected" path resolves to the receiving node itself (the
	// root). The standard client never produces it (it sends "selected"
	// without the dot for the root, which the gate rejects), but the gate
	// accepts it from a hand-crafted frame.
	t.Run("bare .selected path addresses the root", func(t *testing.T) {
		root := newTree()
		if err := root.JawsSetPath(nil, ".selected", true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !root.Selected {
			t.Error("root selected was not set")
		}
		if err := root.JawsSetPath(nil, "selected", true); !errors.Is(err, jawstree.ErrPathRejected) {
			t.Errorf("expected ErrPathRejected for the dotless root path, got %v", err)
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

	// A client Set with an out-of-range child index (index == len) must be rejected
	// without growing Children with a nil node.
	t.Run("rejects out-of-range child index without growing the slice", func(t *testing.T) {
		for _, path := range []string{
			"children.1.selected",            // index == len(Children)
			"children.99.selected",           // far out of range
			"children.-1.selected",           // negative
			"children.x.selected",            // non-numeric
			"children.+0.selected",           // non-canonical: leading '+' (Atoi accepts it)
			"children.00.selected",           // non-canonical: leading zero
			"children.0.children.0.selected", // child has no children
			"bogus.selected",                 // path segment is not "children"
		} {
			root := newTree()
			before := len(root.Children)
			if err := root.JawsSetPath(nil, path, true); !errors.Is(err, jawstree.ErrPathRejected) {
				t.Errorf("path %q: expected ErrPathRejected, got %v", path, err)
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

	// A non-canonical path that still resolves to a valid in-range node (an empty
	// trailing/leading/embedded segment) must be rejected. Such a path otherwise
	// mutates the server node while JawsPathSet echoes the non-canonical path
	// verbatim as the broadcast id, which no peer's rendered (canonical) node id
	// matches, silently desyncing peer selection. "children.0..selected" is the
	// regression: it resolves to Children[0] yet is not the canonical
	// "children.0.selected" that Node.Walk emits.
	t.Run("rejects non-canonical paths resolving to a valid node", func(t *testing.T) {
		for _, path := range []string{
			"children.0..selected", // trailing dot -> nodePath "children.0."
			".children.0.selected", // leading empty segment
			"children..0.selected", // embedded empty segment
		} {
			root := newTree()
			if err := root.JawsSetPath(nil, path, true); !errors.Is(err, jawstree.ErrPathRejected) {
				t.Errorf("path %q: expected ErrPathRejected, got %v", path, err)
			}
			if root.Children[0].Selected {
				t.Errorf("path %q: Children[0].Selected was mutated despite rejection", path)
			}
		}
	})

	// The fix must not reject legitimate canonical paths, including nested ones.
	t.Run("accepts canonical paths including nested", func(t *testing.T) {
		root := &jawstree.Node{Name: "root", Children: []*jawstree.Node{
			{Name: "a", Children: []*jawstree.Node{{Name: "a0"}, {Name: "a1"}}},
			{Name: "b"},
		}}
		if err := root.JawsSetPath(nil, "children.0.children.1.selected", true); err != nil {
			t.Fatalf("nested canonical path: %v", err)
		}
		if !root.Children[0].Children[1].Selected {
			t.Error("nested node was not selected")
		}
		if err := root.JawsSetPath(nil, "children.1.selected", true); err != nil {
			t.Fatalf("canonical path: %v", err)
		}
		if !root.Children[1].Selected {
			t.Error("node was not selected")
		}
	})
}

// TestNode_NilChildGuards exercises the defensive nil-child guards in marshalJSON
// and Walk. A nil child should never occur (the JawsSetPath gate prevents it), but
// both must skip it rather than dereference it.
func TestNode_NilChildGuards(t *testing.T) {
	root := &jawstree.Node{Name: "root", Children: []*jawstree.Node{nil, {Name: "real"}}}

	b, err := root.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("marshalJSON with a nil child produced invalid JSON: %v\n%s", err, b)
	}

	var names []string
	root.Walk("", func(_ string, n *jawstree.Node) { names = append(names, n.Name) })
	if len(names) != 2 || names[0] != "root" || names[1] != "real" {
		t.Errorf("Walk did not skip the nil child: %v", names)
	}
}
