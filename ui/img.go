package ui

import (
	"html/template"
	"io"
	"strconv"

	pkg "github.com/linkdata/jaws/jaws"
)

type Img struct{ pkg.Getter[string] }

func NewImg(g pkg.Getter[string]) *Img { return &Img{Getter: g} }
func (ui *Img) JawsRender(e *pkg.Element, w io.Writer, params []any) (err error) {
	if _, err = e.ApplyGetter(ui.Getter); err == nil {
		srcAttr := template.HTMLAttr("src=" + strconv.Quote(ui.JawsGet(e))) // #nosec G203
		attrs := append(e.ApplyParams(params), srcAttr)
		err = pkg.WriteHTMLInner(w, e.Jid(), "img", "", "", attrs...)
	}
	return
}
func (ui *Img) JawsUpdate(e *pkg.Element) { e.SetAttr("src", ui.JawsGet(e)) }
