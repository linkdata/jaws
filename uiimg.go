package jaws

import (
	"html/template"
	"io"
	"strconv"
	"strings"
)

type UiImg struct {
	UiHtml
	ValueProxy
}

func (ui *UiImg) SrcAttr(e *Element) string {
	var src string
	switch v := ui.ValueProxy.JawsGet(e).(type) {
	case string:
		src = v
	case template.HTML:
		src = string(v)
	default:
		panic("UiImg: src not a string")
	}
	if strings.HasPrefix(src, "\"") {
		return src
	}
	return strconv.Quote(src)
}

func (ui *UiImg) JawsRender(e *Element, w io.Writer, params ...interface{}) {
	ui.ExtractParams(e.Request, ui.ValueProxy, params)
	maybePanic(WriteHtmlInner(w, e.Jid(), "img", "", "", append(ui.Attrs, "src="+ui.SrcAttr(e))...))
}

func (ui *UiImg) JawsUpdate(u Updater) {
	u.SetAttr("src", ui.SrcAttr(u.Element))
}

func NewUiImg(vp ValueProxy) *UiImg {
	return &UiImg{
		UiHtml:     NewUiHtml(),
		ValueProxy: vp,
	}
}

func (rq *Request) Img(imageSrc interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiImg(MakeValueProxy(imageSrc)), params...)
}
