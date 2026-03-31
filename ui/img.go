package ui

import (
	"html/template"
	"io"
	"strconv"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsbind"
	"github.com/linkdata/jaws/jawshtml"
)

type Img struct{ jawsbind.Getter[string] }

func NewImg(g jawsbind.Getter[string]) *Img { return &Img{Getter: g} }
func (rw RequestWriter) Img(imageSrc any, params ...any) error {
	return rw.UI(NewImg(jawsbind.MakeGetter[string](imageSrc)), params...)
}

func (ui *Img) JawsRender(e *jaws.Element, w io.Writer, params []any) (err error) {
	if _, err = e.ApplyGetter(ui.Getter); err == nil {
		srcAttr := template.HTMLAttr("src=" + strconv.Quote(ui.JawsGet(e))) // #nosec G203
		attrs := append(e.ApplyParams(params), srcAttr)
		err = jawshtml.WriteHTMLInner(w, e.Jid(), "img", "", "", attrs...)
	}
	return
}
func (ui *Img) JawsUpdate(e *jaws.Element) { e.SetAttr("src", ui.JawsGet(e)) }
