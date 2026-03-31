package ui

import (
	"html/template"
	"io"
	"strconv"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/bind"
	"github.com/linkdata/jaws/core/htmlx"
)

type Img struct{ bind.Getter[string] }

func NewImg(g bind.Getter[string]) *Img { return &Img{Getter: g} }
func (rw RequestWriter) Img(imageSrc any, params ...any) error {
	return rw.UI(NewImg(bind.MakeGetter[string](imageSrc)), params...)
}

func (ui *Img) JawsRender(e *core.Element, w io.Writer, params []any) (err error) {
	if _, err = e.ApplyGetter(ui.Getter); err == nil {
		srcAttr := template.HTMLAttr("src=" + strconv.Quote(ui.JawsGet(e))) // #nosec G203
		attrs := append(e.ApplyParams(params), srcAttr)
		err = htmlx.WriteHTMLInner(w, e.Jid(), "img", "", "", attrs...)
	}
	return
}
func (ui *Img) JawsUpdate(e *core.Element) { e.SetAttr("src", ui.JawsGet(e)) }
