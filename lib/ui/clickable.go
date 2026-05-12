package ui

import (
	"github.com/linkdata/jaws"
)

// Clickable returns an object implementing [bind.HTMLGetter], [jaws.ClickHandler]
// and tag.TagGetter.
//
// innerHTML is passed to [bind.MakeHTMLGetter], which may or may not provide tags.
//
// Deprecated: use New(innerHTML).Clicked(...) directly.
func Clickable(innerHTML any, onclick func(elem *jaws.Element, click jaws.Click) (err error)) jaws.ClickHandler {
	return New(innerHTML).Clicked(func(_ Object, elem *jaws.Element, click jaws.Click) (err error) {
		return onclick(elem, click)
	})
}
