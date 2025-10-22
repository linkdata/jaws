package jawstree

import (
	"fmt"
	"io"
	"strconv"

	"github.com/linkdata/jaws"
)

var _ jaws.UI = (*Tree)(nil)

type Tree struct {
	id      string // HTML ID of the tree
	options Option
	*jaws.JsVar[Node]
}

func New(id string, jsvar *jaws.JsVar[Node], options ...Option) (t *Tree) {
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
<script>var jawstreeroot_%s; document.addEventListener("DOMContentLoaded",function(){window.jawstree_%s=jawstreeNew("%s",jawstreeroot_%s,%v);});</script>`

func (t *Tree) JawsRender(e *jaws.Element, w io.Writer, params []any) (err error) {
	if err = t.JsVar.JawsRender(e, w, append([]any{"jawstreeroot_" + t.id}, params...)); err == nil {
		if _, err = fmt.Fprintf(w, newtreeTemplate, t.id, t.id, t.id, t.id, t.options); err == nil {
		}
	}
	return
}

func (t *Tree) JawsUpdate(elem *jaws.Element) {
	var b []byte
	b = append(b, `{"tree":`...)
	b = strconv.AppendQuote(b, t.id)
	b = append(b, `,"data":`...)
	b = t.JsVar.Ptr.marshalJSON(b)
	b = append(b, `}`...)
	elem.Jaws.JsCall(t.Tag, "jawstreeSet", string(b))
}
