package jaws

import (
	"html"
	"html/template"
	"sync"
)

var _ StringSetter = UiString{}

// UiString implements StringSetter and HtmlGetter given a sync.Locker (or RLocker) and a string pointer.
type UiString struct {
	L sync.Locker
	P *string
}

func (ui UiString) JawsGetString(e *Element) (val string) {
	if rl, ok := ui.L.(RLocker); ok {
		rl.RLock()
		val = *ui.P
		rl.RUnlock()
		return
	}
	ui.L.Lock()
	val = *ui.P
	ui.L.Unlock()
	return
}

func (ui UiString) JawsSetString(e *Element, val string) (err error) {
	ui.L.Lock()
	*ui.P = val
	ui.L.Unlock()
	return
}

func (ui UiString) JawsGetHtml(e *Element) (val template.HTML) {
	val = template.HTML(html.EscapeString(ui.JawsGetString(e))) // #nosec G203
	return
}
