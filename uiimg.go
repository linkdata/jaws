package jaws

import (
	"html/template"
	"io"
	"strconv"
)

type UiImg struct {
	Getter[string]
}

func (ui *UiImg) JawsRender(e *Element, w io.Writer, params []any) error {
	e.ApplyGetter(ui.Getter)
	srcattr := template.HTMLAttr("src=" + strconv.Quote(ui.JawsGet(e))) // #nosec G203
	attrs := append(e.ApplyParams(params), srcattr)
	return WriteHtmlInner(w, e.Jid(), "img", "", "", attrs...)
}

func (ui *UiImg) JawsUpdate(e *Element) {
	e.SetAttr("src", ui.JawsGet(e))
}

func NewUiImg(g Getter[string]) *UiImg {
	return &UiImg{
		Getter: g,
	}
}

func (rq RequestWriter) Img(imageSrc any, params ...any) error {
	return rq.UI(NewUiImg(makeGetter[string](imageSrc)), params...)
}
