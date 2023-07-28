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

func (ui *UiImg) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiHtmlInner.WriteHtmlInner(rq, w, "img", "", jid, data...)
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
