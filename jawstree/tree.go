package jawstree

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/ui"
)

var _ jaws.UI = (*Tree)(nil)

// Tree renders and updates a shared Quercus.js tree bound to a [ui.JsVar].
//
// A Tree is shared UI state that may be rendered by multiple requests. It embeds
// a [ui.JsVar], which provides the lock and browser communication for the backing
// [Node] tree. Read or mutate that Node tree through Tree methods, or while
// holding the Tree lock.
//
// The tree is structurally fixed once [New] returns: it assigns each node's ID from its
// position, which must match the node's wire position. Mutating node fields (e.g. via
// [Tree.SetSelected]) under the lock is safe, but adding, removing, or reordering Children
// afterward breaks that mapping and is unsupported on a rendered Tree.
type Tree struct {
	id      string // HTML ID of the tree
	options Option
	*ui.JsVar[Node]
}

// New returns a tree widget for jsvar, identified by id.
//
// id must be non-empty and contain only the characters [A-Za-z0-9_$], both jsvar
// and jsvar.Ptr must be non-nil, and the combined options must be non-negative;
// New panics otherwise. Call New before serving or rendering the Tree.
//
// The rendered page must contain an element whose HTML id equals id (for example
// <div id="mytree"></div>): Quercus.js renders the tree into that container. If it
// is missing, the tree silently fails to appear; the only signal is a browser
// console error, with nothing reported server-side.
func New(id string, jsvar *ui.JsVar[Node], options ...Option) (t *Tree) {
	if jsvar == nil {
		panic("jawstree.New: jsvar must not be nil")
	}
	if jsvar.Ptr == nil {
		panic("jawstree.New: jsvar.Ptr must not be nil")
	}
	// id is a URL path segment for the init-script route and the key the browser
	// uses to bracket-index the tree's globals (window["jawstree_"+id] and
	// window["jawstreeroot_"+id]). Validating it here turns what would otherwise be
	// a 400 on the init-script route into an immediate, clear panic.
	if !isSafeTreeName(id) {
		panic("jawstree.New: id must be non-empty and contain only [A-Za-z0-9_$]")
	}
	t = &Tree{
		id:    id,
		JsVar: jsvar,
	}
	for _, opt := range options {
		t.options |= opt
	}
	// options is serialized verbatim into the init-script route, which rejects a
	// negative value (see serveInitScript). Panic here so a bad Option surfaces as a
	// clear construction error rather than a silent 400 and a tree that never renders.
	if t.options < 0 {
		panic("jawstree.New: options must be non-negative")
	}
	// Normalize away any nil children before assigning IDs so a node's slice
	// index (its ID) always matches its position in the compacted wire array
	// emitted by marshalJSON; otherwise a nil child desyncs the two and client
	// path resolution targets the wrong node. See [Node.stripNilChildren].
	jsvar.Ptr.stripNilChildren()
	// The root is identified throughout the name-path API by a nil Parent; the Walk
	// below only sets descendants' Parent, so enforce the invariant for the root here
	// (a node reused as a new root could otherwise carry a stale Parent).
	jsvar.Ptr.Parent = nil
	// Assign each node's JSON-path ID and its owning-Tree and parent back-pointers.
	// The name-path API (Node.HasNames, Node.GetNames, Tree.GetSelected,
	// Tree.SetSelected) requires the parent back-pointers.
	jsvar.Ptr.Walk("", func(jsPath string, node *Node) {
		node.ID = jsPath
		node.Tree = t
		for _, child := range node.Children {
			child.Parent = node // stripNilChildren guarantees no nil entries here
		}
	})
	return
}

// JawsRender renders the hidden root data element and tree initialization script.
func (tree *Tree) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	if err = tree.JsVar.JawsRender(elem, w, append([]any{"jawstreeroot_" + tree.id}, params...)); err == nil {
		var b []byte
		b = append(b, "\n<script"...)
		b = htmlio.AppendAttr(b, "src", initScriptURL(tree.id, tree.options))
		b = append(b, "></script>"...)
		_, err = w.Write(b)
	}
	return
}

// JawsUpdate sends the latest tree JSON to the browser.
//
// It reads the shared Node tree under the Tree read lock, so it is safe to call
// concurrently with the JaWS event goroutines that mutate the tree under the
// write lock.
func (tree *Tree) JawsUpdate(elem *jaws.Element) {
	var b []byte
	b = append(b, `{"tree":`...)
	b = strconv.AppendQuote(b, tree.id)
	b = append(b, `,"data":`...)
	// marshalJSON walks the shared Node tree, which a concurrent JawsInput on
	// another Request can mutate under the JsVar write lock. Read it under the
	// JsVar read lock so the two never race; JawsRender is likewise locked.
	tree.RLock()
	b = tree.JsVar.Ptr.marshalJSON(b)
	tree.RUnlock()
	b = append(b, `}`...)
	elem.JsCall("jawstreeSet", string(b))
}

// Walk calls fn for the tree root and all descendants while holding the tree read lock.
//
// The callback must not call methods that acquire the same tree lock.
func (tree *Tree) Walk(fn func(jsPath string, node *Node)) {
	tree.RLock()
	defer tree.RUnlock()
	tree.Ptr.Walk("", fn)
}

// GetSelected returns selected name-paths while holding the tree read lock.
func (tree *Tree) GetSelected() (nameLists [][]string) {
	tree.RLock()
	defer tree.RUnlock()
	nameLists = tree.Ptr.GetSelected()
	return
}

// SetSelected applies selected name-paths while holding the tree write lock.
//
// The returned [Node] pointers reference the lock-protected shared tree and the
// write lock is released on return, so on a rendered Tree they must only be read
// under the tree read lock (RLock) and mutated under the write lock (Lock), per
// the [Node] concurrency note. Dereferencing them without re-taking the lock
// races the JaWS event goroutines.
func (tree *Tree) SetSelected(nameLists [][]string) (changed []*Node) {
	tree.Lock()
	defer tree.Unlock()
	changed = tree.Ptr.SetSelected(nameLists)
	return
}
