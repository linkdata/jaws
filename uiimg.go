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

func (rq *Request) Img(tagstring, src string, attrs ...interface{}) template.HTML {
	if !strings.HasPrefix(src, "\"") {
		src = strconv.Quote(src)
	}
	attrs = append(attrs, "src="+src)
	ui := &UiImg{
		UiHtmlInner{
			UiHtml: UiHtml{Tags: StringTags(tagstring)},
		},
	}
	return rq.UI(ui, attrs...)
}
