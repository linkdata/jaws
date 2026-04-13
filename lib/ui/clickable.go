package ui

import (
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/jtag"
)

type clickable struct {
	bind.HTMLGetter
	onclick func(elem *jaws.Element, name string) (err error)
}

func (c clickable) JawsClick(elem *jaws.Element, name string) (err error) {
	return c.onclick(elem, name)
}

func (g clickable) JawsGetTag(tc jtag.Context) (tag any) {
	if tg, ok := g.HTMLGetter.(jtag.TagGetter); ok {
		tag = tg.JawsGetTag(tc)
	}
	return
}

// Clickable returns an object implementing bind.HTMLGetter, jaws.ClickHandler and jaws.TagGetter.
// innerHTML is passed to MakeHTMLGetter(), which may or may not provide tags.
func Clickable(innerHTML any, onclick func(elem *jaws.Element, name string) (err error)) jaws.ClickHandler {
	return clickable{HTMLGetter: bind.MakeHTMLGetter(innerHTML), onclick: onclick}
}
