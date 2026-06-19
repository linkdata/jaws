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

var (
	_ ui.SetPather  = (*Node)(nil)
	_ ui.PathSetter = (*Node)(nil)
)

// Node is one tree node rendered by [Tree].
//
// Concurrency: once the owning [Tree] has been rendered, its Node tree is shared
// with the JaWS event goroutines, which access it under the Tree's lock (the
// embedded [ui.JsVar] is an RWLocker). The exported Node accessors below are not
// internally synchronized, so callers must hold that lock when using them on a
// rendered Tree: the Tree's read lock (RLock) for the read-only helpers ([Node.Walk],
// [Node.HasNames], [Node.GetNames], [Node.GetSelected], [Node.MarshalJSON]) and its
// write lock (Lock) for the mutating [Node.SetSelected]. No locking is needed before the
// Tree is rendered (for example while building it in [New]).
//
// marshalJSON is the single source of truth for the wire shape sent to
// Quercus.js; MarshalJSON delegates to it and there is no UnmarshalJSON, so the
// struct json tags below are documentation only and unused for both encoding and
// decoding. They cannot all mirror the wire shape: Disabled is tagged "disabled"
// but is emitted inverted as "selectable":false, so treat marshalJSON as
// authoritative rather than the tags.
type Node struct {
	Tree     *Tree   `json:"-"`                 // owning tree, set by New
	Parent   *Node   `json:"-"`                 // parent node, set by New, nil for root
	Name     string  `json:"name"`              // display name
	ID       string  `json:"id,omitzero"`       // JSON path ID, set by New
	Selected bool    `json:"selected,omitzero"` // selected state
	Disabled bool    `json:"disabled,omitzero"` // emitted as "selectable":false (inverted) on the wire
	Children []*Node `json:"children,omitzero"` // child nodes
}

// appendJSONString appends s as a JSON string literal. Unlike
// [strconv.AppendQuote] it always produces valid JSON: invalid UTF-8 is replaced
// with U+FFFD rather than emitted as a Go-only \xNN escape (which would break
// JSON.parse on the client). Marshaling a string never returns an error.
func appendJSONString(b []byte, s string) []byte {
	enc, _ := json.Marshal(s)
	return append(b, enc...)
}

func (node *Node) marshalJSON(b []byte) []byte {
	b = append(b, `{"name":`...)
	b = appendJSONString(b, node.Name)
	if node.ID != "" {
		b = append(b, `,"id":`...)
		b = appendJSONString(b, node.ID)
	}
	if node.Selected {
		b = append(b, `,"selected":true`...)
	}
	if node.Disabled {
		// Quercus.js expects "selectable"; the server tracks the inverse (Disabled).
		b = append(b, `,"selectable":false`...)
	}
	b = append(b, `,"children":[`...)
	first := true
	for _, c := range node.Children {
		if c == nil { // defensive: the gate in JawsSetPath prevents nil children
			continue
		}
		if !first {
			b = append(b, ',')
		}
		first = false
		b = c.marshalJSON(b)
	}
	b = append(b, "]}"...)
	return b
}

// MarshalJSON writes the Quercus.js JSON shape for node (delegating to the
// canonical marshalJSON encoder).
func (node Node) MarshalJSON() (b []byte, err error) {
	// The receiver is a value, not a pointer, so json.Marshal routes both a Node and a
	// *Node here; a pointer receiver would let a non-addressable Node value fall back to
	// the struct tags and emit a different shape ("disabled":true, not "selectable":false).
	b = node.marshalJSON(nil)
	return
}

var (
	_ json.Marshaler = Node{}
	_ json.Marshaler = (*Node)(nil)
)

// JawsSetPath restricts browser-initiated mutations to the per-node "selected" flag.
//
// Any other path, a non-bool value, or an out-of-range child index is rejected with
// an error matching [ErrPathRejected] without mutating the tree, so a WebSocket
// client cannot change node names, ids, the children slice, or any other [Node]
// field by path. This is the server-side enforcement of the "server holds the truth"
// contract for [Tree].
//
// The root's Selected flag is effectively server-only: the standard client cannot
// produce the path that addresses the root itself, so avoid rendering the root
// selected, since clients cannot change it back through the protocol.
func (node *Node) JawsSetPath(elem *jaws.Element, jsPath string, value any) (err error) {
	// The bare path ".selected" addresses node itself. The standard client never
	// produces it for the root: Quercus.js displays only the root's children, and
	// client-side variable stripping turns the root's own path into "selected"
	// without the dot, which this gate rejects.
	nodePath, ok := strings.CutSuffix(jsPath, ".selected")
	if !ok {
		return fmt.Errorf("%w of %q: only the .selected flag is client-writable", ErrPathRejected, jsPath)
	}
	selected, ok := value.(bool)
	if !ok {
		return fmt.Errorf("%w of %q: expected a bool, got %T", ErrPathRejected, jsPath, value)
	}
	// Resolve the path ourselves with strict in-range index bounds rather than
	// delegating to the generic JsVar path-setter (jq.Set), which would set
	// arbitrary json-tagged fields and grow a slice by one when asked to set
	// index == len.
	var target *Node
	if target, err = node.resolveChildPath(nodePath); err == nil {
		if target.Selected == selected {
			err = jaws.ErrValueUnchanged
		} else {
			target.Selected = selected
		}
	}
	return
}

// resolveChildPath navigates from node following a canonical path of the form
// "children.<i>.children.<j>..."; an empty path resolves to node itself.
//
// Every index must be within the current Children range and the whole path must be
// canonical; out-of-range, malformed, non-canonical or nil-targeting segments are
// rejected with [ErrPathRejected], so a client can neither grow the slice nor
// address a node that does not exist.
func (node *Node) resolveChildPath(nodePath string) (*Node, error) {
	// The path must be canonical so it round-trips to the exact string Node.Walk
	// emits as the node ID: segments strictly alternate "children" and a canonical
	// decimal index (no leading '+', '-' or zeros), joined by single dots with no
	// empty segment from a leading, trailing or doubled '.'. A non-canonical path
	// that still resolved to a valid in-range node ("children.0.", "children.+0")
	// would mutate the server node yet be echoed verbatim as the Node.JawsPathSet
	// broadcast "id", which no peer's rendered node matches, diverging peer state.
	cur := node
	for rest := nodePath; rest != ""; {
		var seg, idxStr string
		var more bool
		seg, rest, _ = strings.Cut(rest, ".")
		if seg != "children" {
			return nil, fmt.Errorf("%w: unexpected path segment %q", ErrPathRejected, seg)
		}
		idxStr, rest, more = strings.Cut(rest, ".")
		idx, err := strconv.Atoi(idxStr)
		if err != nil || idx < 0 || idx >= len(cur.Children) || cur.Children[idx] == nil {
			return nil, fmt.Errorf("%w: child index %q out of range", ErrPathRejected, idxStr)
		}
		// strconv.Itoa is allocation-free for indices below 100. The trailing-dot
		// case ("children.0.") is the one a per-index check alone misses: it leaves
		// an empty final segment the loop never visits, so reject when Cut reported
		// a separator was consumed ('more') but nothing follows it.
		if strconv.Itoa(idx) != idxStr || (more && rest == "") {
			return nil, fmt.Errorf("%w: non-canonical path %q", ErrPathRejected, nodePath)
		}
		cur = cur.Children[idx]
	}
	return cur, nil
}

// JawsPathSet runs after a node's selected flag has been set on the server-side
// tree; it broadcasts a jawstreeSetPath JsCall so the change is reflected in the
// rendered tree of every client sharing this Tree.
func (node *Node) JawsPathSet(elem *jaws.Element, jsPath string, value any) {
	if nodePath, ok := strings.CutSuffix(jsPath, ".selected"); ok {
		payload, _ := json.Marshal(struct {
			Tree string `json:"tree"`
			ID   string `json:"id"`
			Set  any    `json:"set"`
		}{node.Tree.id, nodePath, value})
		elem.Jaws.JsCall(node.Tree.JawsGetTag(nil), "jawstreeSetPath", string(payload))
	}
}

// stripNilChildren removes any nil entries from node.Children and every
// descendant's Children, in place.
//
// [New] calls it before assigning IDs so each node's slice index (its ID) matches
// its position in the compacted wire array; see the rationale at the call site.
func (node *Node) stripNilChildren() {
	node.Children = slices.DeleteFunc(node.Children, func(c *Node) bool { return c == nil })
	for _, child := range node.Children {
		child.stripNilChildren()
	}
}

// Walk calls fn for node and all descendants with their JSON paths.
//
// node is visited with the supplied jsPath; callers pass "" for the root. Each
// descendant is visited with "children.<i>" appended to its parent's path, where i
// is the child's index in node.Children. A nil child is skipped while i keeps the
// raw slice index, so on a Tree built by [New] — where [Node.stripNilChildren] has
// already removed nil entries — these indices stay dense and match the wire
// positions emitted by [Node.marshalJSON].
func (node *Node) Walk(jsPath string, fn func(jsPath string, node *Node)) {
	fn(jsPath, node)
	if jsPath != "" {
		jsPath += "."
	}
	for i, child := range node.Children {
		if child == nil { // defensive: the gate in JawsSetPath prevents nil children
			continue
		}
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

// GetSelected returns the name-paths (root-to-node name lists) of all selected nodes.
//
// Selection is reported and matched by name-path, not by the unique node identity
// used on the wire. If sibling nodes share the same name their name-paths are
// identical, so the round-trip is lossy: [Node.SetSelected] cannot tell them apart
// and will select every sibling sharing a selected name-path. Give siblings
// distinct names if they must be addressed independently through this API.
func (node *Node) GetSelected() (nameLists [][]string) {
	node.Walk("", func(jsPath string, node *Node) {
		if node.Selected {
			nameLists = append(nameLists, node.GetNames())
		}
	})
	return
}

// SetSelected applies the given selected name-paths and returns the nodes that changed.
//
// Nodes are matched by name-path (see [Node.GetSelected]); when sibling nodes
// share a name they are selected or deselected together, since their name-paths
// are indistinguishable.
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
