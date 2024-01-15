package jaws

import (
	"html/template"
	"io"
	"strconv"
)

type UiImg struct {
	UiHtml
	StringSetter
}

func (ui *UiImg) SrcAttr(e *Element) string {
	src := ui.JawsGetString(e)
	if len(src) < 1 || src[0] != '"' {
		return strconv.Quote(src)
	}
	return src
}

func (ui *UiImg) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	ui.parseGetter(e, ui.StringSetter)
	srcattr := template.HTMLAttr("src=" + ui.SrcAttr(e)) // #nosec G203
	attrs := append(e.ParseParams(params), srcattr)
	return WriteHtmlInner(w, e.Jid(), "img", "", "", attrs...)
}

func (ui *UiImg) JawsUpdate(u *Element) {
	u.SetAttr("src", ui.SrcAttr(u))
}

func NewUiImg(g StringSetter) *UiImg {
	return &UiImg{
		StringSetter: g,
	}
}

func (rq RequestWriter) Img(imageSrc interface{}, params ...interface{}) error {
	return rq.UI(NewUiImg(makeStringSetter(imageSrc)), params...)
}
