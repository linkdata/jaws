package jawstree_test

import (
	"encoding/json"
	"testing"

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

	// MarshalJSON's value receiver makes json.Marshal route both a *Node and a Node
	// value through the canonical encoder; a pointer receiver would let a value fall
	// back to the struct tags and emit "disabled":true instead of "selectable":false.
	for _, v := range []any{rootnode, *rootnode} {
		got, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("json.Marshal(%T): %v", v, err)
		}
		if string(got) != want {
			t.Errorf("json.Marshal(%T):\n got %s\nwant %s\n", v, got, want)
		}
	}
}

// TestNode_MarshalJSON_AdversarialNames ensures node names that are not clean ASCII
// (invalid UTF-8 from a raw filesystem entry, embedded quotes/newlines) still produce
// valid JSON that the browser's JSON.parse accepts: appendJSONString must emit
// JSON-compatible escapes, never Go-only forms such as \xNN.
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

// TestNode_NilChildGuards exercises the defensive nil-child guards in marshalJSON and
// Walk. A nil child should never occur (New strips them), but both must skip it rather
// than dereference it.
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
