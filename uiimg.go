package jaws

import (
	"html/template"
	"io"
	"strconv"
	"strings"
)

type UiImg struct {
	UiHtmlInner
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
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "img", "", append(e.Data, "src="+ui.SrcAttr(e)))
}

func (ui *UiImg) JawsUpdate(e *Element) (err error) {
	if e.SetAttr("src", ui.SrcAttr(e)) {
		e.UpdateOthers(ui.Tags)
	}
	return nil
}

func NewUiImg(up Params) *UiImg {
	return &UiImg{
		NewUiHtmlInner(up),
	}
}

func (rq *Request) Img(params ...interface{}) template.HTML {
	return rq.UI(NewUiImg(NewParams(params)), params...)
}
