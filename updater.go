package jaws

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/linkdata/jaws/what"
)

type Updater struct {
	outCh chan<- wsMsg
	order uint64
	*Element
}

func (u *Updater) send(wht what.What, data string) {
	u.Element.send(u.outCh, wsMsg{
		Jid:  u.jid,
		What: wht,
		Data: data,
	})
}

// SetAttr queues sending a new attribute value
// to the browser for the Element with the given JaWS ID in this Request.
func (u *Updater) SetAttr(attr, val string) {
	u.send(what.SAttr, attr+"\n"+val)
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the Element with the given JaWS ID in this Request.
func (u *Updater) RemoveAttr(attr string) {
	u.send(what.RAttr, attr)
}

// SetInner queues sending a new inner HTML content
// to the browser for the Element.
func (u *Updater) SetInner(innerHtml template.HTML) {
	u.send(what.Inner, string(innerHtml))
}

// SetValue queues sending a new current input value in textual form
// to the browser for the Element with the given JaWS ID in this Request.
func (u *Updater) SetValue(val string) {
	u.send(what.Value, val)
}

// Replace replaces the elements entire HTML DOM node with new HTML code.
// If the HTML code doesn't seem to contain correct HTML ID, it panics.
func (u *Updater) Replace(htmlCode template.HTML) {
	var b []byte
	b = append(b, "id="...)
	b = u.Jid().AppendQuote(b)
	if !bytes.Contains([]byte(htmlCode), b) {
		panic(fmt.Errorf("jaws: Updater.Replace(): expected HTML " + string(b)))
	}
	u.send(what.Replace, string(htmlCode))
}
