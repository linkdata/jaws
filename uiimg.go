package jaws

import (
	"html/template"
	"io"
	"strconv"
)

type UiImg struct {
	UiGetter
}

func (ui *UiImg) SrcAttr(e *Element) string {
	var src string
	switch v := ui.JawsGet(e).(type) {
	case string:
		src = v
	case template.HTML:
		src = string(v)
	default:
		panic("UiImg: src not a string")
	}
	if len(src) < 1 || src[0] != '"' {
		return strconv.Quote(src)
	}
	return src
}

func (ui *UiImg) JawsRender(e *Element, w io.Writer, params []interface{}) {
	attrs := append(ui.parseParams(e, params), "src="+ui.SrcAttr(e))
	maybePanic(WriteHtmlInner(w, e.Jid(), "img", "", "", attrs...))
}

func (ui *UiImg) JawsUpdate(u Updater) {
	u.SetAttr("src", ui.SrcAttr(u.Element))
}

func NewUiImg(vp Getter) *UiImg {
	return &UiImg{
		UiGetter{
			Getter: vp,
		},
	}
}

func (rq *Request) Img(imageSrc interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiImg(MakeGetter(imageSrc)), params...)
}
