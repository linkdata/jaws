package ui

import (
	"html/template"
	"io"
	"strconv"

	"github.com/linkdata/jaws/core"
)

type Img struct{ core.Getter[string] }

func NewImg(g core.Getter[string]) *Img { return &Img{Getter: g} }
func (rw RequestWriter) Img(imageSrc any, params ...any) error {
	return rw.UI(NewImg(core.MakeGetter[string](imageSrc)), params...)
}

func (ui *Img) JawsRender(e *core.Element, w io.Writer, params []any) (err error) {
	if _, err = e.ApplyGetter(ui.Getter); err == nil {
		srcAttr := template.HTMLAttr("src=" + strconv.Quote(ui.JawsGet(e))) // #nosec G203
		attrs := append(e.ApplyParams(params), srcAttr)
		err = core.WriteHTMLInner(w, e.Jid(), "img", "", "", attrs...)
	}
	return
}
func (ui *Img) JawsUpdate(e *core.Element) { e.SetAttr("src", ui.JawsGet(e)) }
