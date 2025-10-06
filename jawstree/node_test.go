package jawstree_test

import (
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
}
