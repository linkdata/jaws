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
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "img", "")
}

func NewUiImg(tags []interface{}) *UiImg {
	return &UiImg{
		UiHtmlInner{
			UiHtml: UiHtml{Tags: tags},
		},
	}
}

func (rq *Request) Img(tagitem interface{}, src string, data ...interface{}) template.HTML {
	if !strings.HasPrefix(src, "\"") {
		src = strconv.Quote(src)
	}
	data = append(data, "src="+src)
	return rq.UI(NewUiImg(ProcessTags(tagitem)), data...)
}
