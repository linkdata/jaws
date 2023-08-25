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

func (ui *UiImg) JawsRender(e *Element, w io.Writer) error {
	src := anyToHtml(ui.ValueReader.JawsGet(e))
	if !strings.HasPrefix(src, "\"") {
		src = strconv.Quote(src)
	}
	data := append(e.Data, "src="+src)
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "img", "", data)
}

func (ui *UiImg) JawsUpdate(e *Element) (err error) {
	src := anyToHtml(ui.ValueReader.JawsGet(e))
	if !strings.HasPrefix(src, "\"") {
		src = strconv.Quote(src)
	}
	if e.SetAttr("src", src) {
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
