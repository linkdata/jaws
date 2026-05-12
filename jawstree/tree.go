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
	jsvar.Ptr.Walk("", func(jspath string, n *Node) { n.ID = jspath; n.Tree = t })
	return
}

const newtreeTemplate = `
<script src=%q></script>`

// JawsRender renders the hidden root data element and tree initialization script.
func (t *Tree) JawsRender(e *jaws.Element, w io.Writer, params []any) (err error) {
	if err = t.JsVar.JawsRender(e, w, append([]any{"jawstreeroot_" + t.id}, params...)); err == nil {
		if _, err = fmt.Fprintf(w, newtreeTemplate, initScriptURL(t.id, t.options)); err == nil {
		}
	}
	return
}

// JawsUpdate sends the latest tree JSON to the browser.
func (t *Tree) JawsUpdate(elem *jaws.Element) {
	var b []byte
	b = append(b, `{"tree":`...)
	b = strconv.AppendQuote(b, t.id)
	b = append(b, `,"data":`...)
	b = t.JsVar.Ptr.marshalJSON(b)
	b = append(b, `}`...)
	elem.Jaws.JsCall(t.Tag, "jawstreeSet", string(b))
}
