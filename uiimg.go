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
		panic("jaws: UiImg: src not a string")
	}
	if strings.HasPrefix(src, "\"") {
		return src
	}
	return strconv.Quote(src)
}

func (ui *UiImg) JawsRender(e *Element, w io.Writer) error {
	return WriteHtmlInner(w, e.Jid(), "img", "", "", append(e.Attrs(), "src="+ui.SrcAttr(e))...)
}

func (ui *UiImg) JawsUpdate(e *Element, u Updater) (err error) {
	u.SetAttr("src", ui.SrcAttr(e))
	return nil
}

func NewUiImg(up Params) *UiImg {
	return &UiImg{
		UiHtml:     NewUiHtml(up),
		ValueProxy: up.ValueProxy(),
	}
}

func (rq *Request) Img(imageSrc interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiImg(NewParams(imageSrc, params)), params...)
}
