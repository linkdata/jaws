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
	src := string(ui.InnerProxy.JawsInner(e))
	if !strings.HasPrefix(src, "\"") {
		src = strconv.Quote(src)
	}
	data := append(e.Data, "src="+src)
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "img", "", data)
}

func (ui *UiImg) JawsUpdate(e *Element) (err error) {
	src := string(ui.InnerProxy.JawsInner(e))
	if !strings.HasPrefix(src, "\"") {
		src = strconv.Quote(src)
	}
	if e.SetAttr("src", src) {
		e.UpdateOthers(ui.Tags)
	}
	return nil
}

func NewUiImg(tags []interface{}, inner InnerProxy) *UiImg {
	return &UiImg{
		NewUiHtmlInner(tags, inner),
	}
}

func (rq *Request) Img(tagitem interface{}, src interface{}, data ...interface{}) template.HTML {
	return rq.UI(NewUiImg(ProcessTags(tagitem), MakeInnerProxy(src)), data...)
}
