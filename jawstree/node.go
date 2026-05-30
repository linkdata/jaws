package jawstree

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/ui"
)

var _ ui.SetPather = (*Node)(nil)

// Node is one tree node rendered by [Tree].
//
// Concurrency: once the owning [Tree] has been rendered, its Node tree is shared
// with the JaWS event goroutines, which access it under the Tree's lock (the
// embedded [ui.JsVar] is an RWLocker). The exported Node accessors below are not
// internally synchronized, so callers must hold that lock when using them on a
// rendered Tree: the Tree's read lock (RLock) for the read-only helpers ([Node.Walk],
// [Node.HasNames], [Node.GetNames], [Node.GetSelected]) and its write lock (Lock)
// for the mutating [Node.SetSelected]. No locking is needed before the Tree is
// rendered (for example while building it in [New]).
//
// marshalJSON is the single source of truth for the wire shape sent to
// Quercus.js; MarshalJSON delegates to it, so the struct json tags below are not
// actually used for encoding and must be kept in sync with marshalJSON by hand.
type Node struct {
	Tree     *Tree   `json:"-"`                 // owning tree, set by New
	Parent   *Node   `json:"-"`                 // parent node, nil for root
	Name     string  `json:"name"`              // display name
	ID       string  `json:"id,omitzero"`       // JSON path ID, set by New
	Selected bool    `json:"selected,omitzero"` // selected state
	Disabled bool    `json:"disabled,omitzero"` // emitted as "selectable":false (inverted) on the wire
	Children []*Node `json:"children,omitzero"` // child nodes
}

func (node *Node) marshalJSON(b []byte) []byte {
	b = append(b, `{"name":`...)
	b = strconv.AppendQuote(b, node.Name)
	if node.ID != "" {
		b = append(b, `,"id":`...)
		b = strconv.AppendQuote(b, node.ID)
	}
	if node.Selected {
		b = append(b, `,"selected":true`...)
	}
	if node.Disabled {
		// Quercus.js expects "selectable"; the server tracks the inverse (Disabled).
		b = append(b, `,"selectable":false`...)
	}
	b = append(b, `,"children":[`...)
	for i, c := range node.Children {
		if i > 0 {
			b = append(b, ',')
		}
		b = c.marshalJSON(b)
	}
	b = append(b, "]}"...)
	return b
}

// MarshalJSON writes the Quercus.js JSON shape for node (delegating to the
// canonical marshalJSON encoder).
func (node *Node) MarshalJSON() (b []byte, err error) {
	b = node.marshalJSON(nil)
	return
}

var _ json.Marshaler = &Node{}

// JawsPathSet mirrors browser-side selected-state changes back into the tree.
func (node *Node) JawsPathSet(elem *jaws.Element, jsPath string, value any) {
	if nodePath, ok := strings.CutSuffix(jsPath, ".selected"); ok {
		elem.Jaws.JsCall(node.Tree.JawsGetTag(nil), "jawstreeSetPath", fmt.Sprintf(`{"tree":%q,"id":%q,"set":%v}`, node.Tree.id, nodePath, value))
	}
}

// Walk calls fn for node and all descendants with their JSON paths.
func (node *Node) Walk(jsPath string, fn func(jsPath string, node *Node)) {
	fn(jsPath, node)
	if jsPath != "" {
		jsPath += "."
	}
	for i, child := range node.Children {
		child.Walk(jsPath+"children."+strconv.Itoa(i), fn)
	}
}

// HasNames reports whether node matches names as a path from the root.
func (node *Node) HasNames(names []string) (yes bool) {
	if yes = (node.Parent == nil) && (len(names) == 0); !yes && node.Parent != nil {
		if len(names) > 0 {
			yes = node.Parent.HasNames(names[:len(names)-1])
			yes = yes && node.Name == names[len(names)-1]
		}
	}
	return
}

// GetNames returns the path of names from the root to node.
func (node *Node) GetNames() (names []string) {
	for node.Parent != nil {
		names = append(names, node.Name)
		node = node.Parent
	}
	slices.Reverse(names)
	return
}

// GetSelected returns the paths of all selected nodes.
func (node *Node) GetSelected() (nameLists [][]string) {
	node.Walk("", func(jsPath string, node *Node) {
		if node.Selected {
			nameLists = append(nameLists, node.GetNames())
		}
	})
	return
}

// SetSelected applies selected paths and returns nodes that changed.
//
// It mutates the shared Node tree; on a rendered [Tree], hold the Tree's write
// lock while calling it (see the [Node] concurrency note).
func (node *Node) SetSelected(nameLists [][]string) (changed []*Node) {
	node.Walk("", func(jsPath string, node *Node) {
		selected := false
		for _, names := range nameLists {
			if selected = node.HasNames(names); selected {
				break
			}
		}
		if selected != node.Selected {
			node.Selected = selected
			changed = append(changed, node)
		}
	})
	return
}
