package ui

import (
	"html/template"
	"io"
	"strconv"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
)

type Img struct{ bind.Getter[string] }

func NewImg(g bind.Getter[string]) *Img { return &Img{Getter: g} }
func (rw RequestWriter) Img(imageSrc any, params ...any) error {
	return rw.UI(NewImg(bind.MakeGetter[string](imageSrc)), params...)
}

func (ui *Img) JawsRender(e *jaws.Element, w io.Writer, params []any) (err error) {
	var getterAttrs []template.HTMLAttr
	if _, getterAttrs, err = e.ApplyGetter(ui.Getter); err == nil {
		srcAttr := template.HTMLAttr("src=" + strconv.Quote(ui.JawsGet(e))) // #nosec G203
		attrs := append(e.ApplyParams(params), getterAttrs...)
		attrs = append(attrs, srcAttr)
		err = htmlio.WriteHTMLInner(w, e.Jid(), "img", "", "", attrs...)
	}
	return
}
func (ui *Img) JawsUpdate(e *jaws.Element) { e.SetAttr("src", ui.JawsGet(e)) }
