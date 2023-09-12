package jaws

import (
	"bytes"
	"html/template"

	"github.com/linkdata/jaws/what"
)

type Updater struct {
	ch  chan<- wsMsg
	jid Jid
}

// SetAttr queues sending a new attribute value
// to the browser for the Element with the given JaWS ID in this Request.
func (u Updater) SetAttr(attr, val string) {
	u.ch <- wsMsg{
		Jid:  u.jid,
		Data: attr + "\n" + val,
		What: what.SAttr,
	}
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the Element with the given JaWS ID in this Request.
func (u Updater) RemoveAttr(attr string) {
	u.ch <- wsMsg{
		Jid:  u.jid,
		Data: attr,
		What: what.RAttr,
	}
}

// SetInner queues sending a new inner HTML content
// to the browser for the Element.
func (u Updater) SetInner(innerHtml template.HTML) {
	u.ch <- wsMsg{
		Jid:  u.jid,
		Data: string(innerHtml),
		What: what.Inner,
	}
}

// SetValue queues sending a new current input value in textual form
// to the browser for the Element with the given JaWS ID in this Request.
func (u Updater) SetValue(val string) {
	u.ch <- wsMsg{
		Jid:  u.jid,
		Data: val,
		What: what.Value,
	}
}

// Replace replaces the elements entire HTML DOM node with new HTML code.
// If the HTML code doesn't seem to contain correct HTML ID, it panics.
func (u Updater) Replace(htmlCode template.HTML) {
	var b []byte
	b = append(b, "id="...)
	b = u.jid.AppendQuote(b)
	if !bytes.Contains([]byte(htmlCode), b) {
		panic("jaws: Updater.Replace(): expected HTML " + string(b))
	}
	u.ch <- wsMsg{
		Jid:  u.jid,
		Data: string(htmlCode),
		What: what.Replace,
	}
}
