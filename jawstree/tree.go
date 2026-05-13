package jawstree

import (
	"fmt"
	"io"
	"strconv"

	"github.com/linkdata/jaws"
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
// New initializes node IDs and tree back-pointers in jsvar.Ptr.
func New(id string, jsvar *ui.JsVar[Node], options ...Option) (t *Tree) {
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

const newtreeTemplate = `
<script src=%q></script>`

// JawsRender renders the hidden root data element and tree initialization script.
func (tree *Tree) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	if err = tree.JsVar.JawsRender(elem, w, append([]any{"jawstreeroot_" + tree.id}, params...)); err == nil {
		if _, err = fmt.Fprintf(w, newtreeTemplate, initScriptURL(tree.id, tree.options)); err == nil {
		}
	}
	return
}

// JawsUpdate sends the latest tree JSON to the browser.
func (tree *Tree) JawsUpdate(elem *jaws.Element) {
	var b []byte
	b = append(b, `{"tree":`...)
	b = strconv.AppendQuote(b, tree.id)
	b = append(b, `,"data":`...)
	b = tree.JsVar.Ptr.marshalJSON(b)
	b = append(b, `}`...)
	elem.Jaws.JsCall(tree.Tag, "jawstreeSet", string(b))
}
