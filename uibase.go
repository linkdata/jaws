package jaws

import (
	"io"
	"strings"

	"github.com/linkdata/jaws/what"
)

type UiBase struct {
	Tags    string
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

func (uib *UiBase) JawsTags(rq *Request) (tags []interface{}) {
	return StringTags(uib.Tags)
}

func (uib *UiBase) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) (err error) {
	return
}

func (uib *UiBase) JawsEvent(rq *Request, wht what.What, jid string, val string) (err error) {
	if uib.EventFn != nil {
		err = uib.EventFn(rq, wht, jid, val)
	}
	return
}
