package jawstree

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/ui"
)

var _ jaws.UI = (*Tree)(nil)

// Tree renders and updates a Quercus.js tree bound to a [ui.JsVar].
type Tree struct {
	id      string // HTML ID of the tree
	options Option
	*ui.JsVar[Node]
}

// New returns a tree widget with id, jsvar and options.
//
// The id is used both as a JavaScript variable name and as a URL path segment
// for the init script, so it must be non-empty and contain only the characters
// [A-Za-z0-9_$]; otherwise New panics. Validating here turns what would
// otherwise be a confusing render-time "illegal jsvar name" error and a 400 on
// the init-script route into an immediate, clear failure.
//
// New initializes node IDs and tree back-pointers in jsvar.Ptr.
// It panics if jsvar or jsvar.Ptr is nil, or if id is not a valid name.
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
	jsvar.Ptr.Walk("", func(jsPath string, node *Node) { node.ID = jsPath; node.Tree = t })
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
	elem.Jaws.JsCall(tree.JawsGetTag(nil), "jawstreeSet", string(b))
}
