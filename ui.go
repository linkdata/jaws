package jaws

import (
	"bytes"
	"html/template"
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UI interface {
	JawsTags(rq *Request) (tl []interface{})
	JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) (err error)
	JawsEvent(rq *Request, wht what.What, jid, val string) (err error)
}

func (rq *Request) newElementLocked(tags []interface{}, ui UI, data ...interface{}) (elem *Element) {
	if len(tags) > 0 {
		rq.nextJid++
		jid := " " + strconv.Itoa(rq.nextJid)
		elem = &Element{Jid: jid, Ui: ui, Data: data}
		rq.elems = append(rq.elems, elem)
		rq.tagMap[jid] = append(rq.tagMap[jid], elem)
		for _, tag := range tags {
			rq.tagMap[tag] = append(rq.tagMap[tag], elem)
		}
	}
	return
}

func (rq *Request) UI(ui UI, data ...interface{}) template.HTML {
	tags := ui.JawsTags(rq)
	rq.mu.Lock()
	elem := rq.newElementLocked(tags, ui, data)
	rq.mu.Unlock()
	var b bytes.Buffer
	if err := ui.JawsRender(rq, &b, elem.Jid, data...); err != nil {
		rq.Jaws.MustLog(err)
	}
	return template.HTML(b.String())
}

func (rq *Request) Render(tags []interface{}) {
	var todo []*Element
	rq.mu.RLock()
	for _, tag := range tags {
		todo = append(todo, rq.tagMap[tag]...)
	}
	rq.mu.RUnlock()
	var b bytes.Buffer
	for _, ui := range todo {
		if err := ui.Ui.JawsRender(rq, &b, ui.Jid, ui.Data...); err != nil {
			rq.Jaws.MustLog(err)
		}
		b.Reset()
	}
}

type Ui interface {
	JawsUi(rq *Request, attrs ...string) template.HTML
}

func (rq *Request) Ui(elem Ui, attrs ...string) template.HTML {
	return elem.JawsUi(rq, attrs...)
}
