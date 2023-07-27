package jaws

import (
	"io"
	"strings"

	"github.com/linkdata/jaws/what"
)

type UiHtml struct {
	Tags    []interface{}
	EventFn EventFn
}

func StringTags(text string) (tags []interface{}) {
	for _, s := range strings.Split(text, " ") {
		if s != "" {
			tags = append(tags, s)
		}
	}
	return
}

func (ui *UiHtml) WriteHtmlInner(rq *Request, w io.Writer, htmltag, htmltype, htmlinner, jid string, data ...interface{}) error {
	var attrs []string
	for _, v := range data {
		if s, ok := v.(string); ok {
			attrs = append(attrs, s)
		}
	}
	return WriteHtmlInner(w, jid, htmltag, htmltype, htmlinner, attrs...)
}

func (uib *UiHtml) JawsTags(rq *Request) (tags []interface{}) {
	return uib.Tags
}

func (uib *UiHtml) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) (err error) {
	return
}

func (uib *UiHtml) JawsEvent(rq *Request, wht what.What, jid string, val string) (err error) {
	if uib.EventFn != nil {
		err = uib.EventFn(rq, wht, jid, val)
	}
	return
}
