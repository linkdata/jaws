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
type Tree struct {
	id      string // HTML ID of the tree
	options Option
	*ui.JsVar[Node]
}

// New returns a tree widget with id, jsvar and options.
//
// The id is a URL path segment for the init-script route and the key the browser
// uses to bracket-index the tree's globals (window["jawstree_"+id] and
// window["jawstreeroot_"+id]), so it must be non-empty and contain only the
// characters [A-Za-z0-9_$]; otherwise New panics. Validating here turns what would
// otherwise be a 400 on the init-script route into an immediate, clear failure.
//
// New initializes node IDs, the owning Tree back-pointer and parent
// back-pointers in jsvar.Ptr; the name-path API ([Node.HasNames],
// [Node.GetNames], [Tree.GetSelected], [Tree.SetSelected]) requires the parent
// back-pointers.
// It panics if jsvar or jsvar.Ptr is nil, or if id is not a valid name.
// Call New before serving or rendering the Tree.
//
// The rendered page must contain an element whose HTML id equals id (for
// example <div id="mytree"></div>): Quercus.js renders the tree into that
// container. If it is missing, the tree silently fails to appear; the only
// signal is a browser console error, with nothing reported server-side.
func New(id string, jsvar *ui.JsVar[Node], options ...Option) (t *Tree) {
	if jsvar == nil {
		panic("jawstree.New: jsvar must not be nil")
	}
	if jsvar.Ptr == nil {
		panic("jawstree.New: jsvar.Ptr must not be nil")
	}
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
	// Normalize away any nil children before assigning IDs so a node's slice
	// index (its ID) always matches its position in the compacted wire array
	// emitted by marshalJSON; otherwise a nil child desyncs the two and client
	// path resolution targets the wrong node. See [Node.stripNilChildren].
	jsvar.Ptr.stripNilChildren()
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
func (tree *Tree) SetSelected(nameLists [][]string) (changed []*Node) {
	tree.Lock()
	defer tree.Unlock()
	changed = tree.Ptr.SetSelected(nameLists)
	return
}
