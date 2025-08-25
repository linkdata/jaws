package jawstree

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/linkdata/jaws"
)

var _ jaws.SetPather = (*Node)(nil)

type Node struct {
	Tree     *Tree   `json:"-"`
	Parent   *Node   `json:"-"`
	ID       string  `json:"id,omitzero"`
	Name     string  `json:"name"`
	Selected bool    `json:"selected,omitzero"`
	Children []*Node `json:"children,omitzero"`
}

func (n *Node) JawsPathSet(elem *jaws.Element, jspath string, value any) {
	if jspath, ok := strings.CutSuffix(jspath, ".selected"); ok {
		elem.Jaws.JsCall(n.Tree.Tag, "jawstreeSetPath", fmt.Sprintf(`{"tree":%q,"id":%q,"set":%v}`, n.Tree.id, jspath, value))
	}
}

func (n *Node) Walk(jspath string, fn func(jspath string, node *Node)) {
	fn(jspath, n)
	if jspath != "" {
		jspath += "."
	}
	for i, child := range n.Children {
		child.Walk(jspath+"children."+strconv.Itoa(i), fn)
	}
}

func (n *Node) HasNames(names []string) (yes bool) {
	if yes = (n.Parent == nil) && (len(names) == 0); !yes && n.Parent != nil {
		if len(names) > 0 {
			yes = n.Parent.HasNames(names[:len(names)-1])
			yes = yes && n.Name == names[len(names)-1]
		}
	}
	return
}

func (n *Node) GetNames() (names []string) {
	for n.Parent != nil {
		names = append(names, n.Name)
		n = n.Parent
	}
	slices.Reverse(names)
	return
}

func (n *Node) GetSelected() (nameslist [][]string) {
	n.Walk("", func(jspath string, node *Node) {
		if node.Selected {
			nameslist = append(nameslist, node.GetNames())
		}
	})
	return
}

func (n *Node) SetSelected(nameslist [][]string) (changed []*Node) {
	n.Walk("", func(jspath string, node *Node) {
		for _, names := range nameslist {
			if selected := node.HasNames(names); selected != node.Selected {
				node.Selected = selected
				changed = append(changed, node)
			}
		}
	})
	return
}
