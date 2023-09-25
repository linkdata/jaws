package jaws

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/linkdata/jaws/what"
)

type Updater struct {
	outCh chan<- wsMsg
	*Element
}

func (u *Updater) send(wht what.What, data string) {
	u.Request.send(u.outCh, wsMsg{
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

// Append appends a new HTML element as a child to the current one.
func (u *Updater) Append(htmlCode template.HTML) {
	u.send(what.Append, string(htmlCode))
}

// Order reorders the HTML child elements of the current Element.
func (u *Updater) Order(jidList []Jid) {
	if len(jidList) > 0 {
		var b []byte
		for i, jid := range jidList {
			if i > 0 {
				b = append(b, ' ')
			}
			b = jid.AppendInt(b)
		}
		u.send(what.Order, string(b))
	}
}

// Remove removes the HTML element with the given Jid.
func (u *Updater) Remove(jid Jid) {
	u.Request.send(u.outCh, wsMsg{
		Jid:  jid,
		What: what.Remove,
	})
}
