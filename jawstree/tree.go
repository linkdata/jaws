package jawstree

import (
	"fmt"
	"io"

	"github.com/linkdata/jaws"
)

var _ jaws.UI = (*Tree)(nil)

type Tree struct {
	id string // HTML ID of the tree
	*jaws.JsVar[Node]
}

func New(id string, jsvar *jaws.JsVar[Node]) (t *Tree) {
	t = &Tree{
		id:    id,
		JsVar: jsvar,
	}
	jsvar.Ptr.Walk("", func(jspath string, n *Node) { n.ID = jspath; n.Tree = t })
	return
}

const newtreeTemplate = `
<script>var jawstreeroot_%s; document.addEventListener("DOMContentLoaded",function(){window.jawstree_%s=jawstreeNew("%s",jawstreeroot_%s);});</script>`

func (t *Tree) JawsRender(e *jaws.Element, w io.Writer, params []any) (err error) {
	if err = t.JsVar.JawsRender(e, w, []any{"jawstreeroot_" + t.id}); err == nil {
		if _, err = fmt.Fprintf(w, newtreeTemplate, t.id, t.id, t.id, t.id); err == nil {
		}
	}
	return
}
