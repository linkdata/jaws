package ui

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
)

// Img renders an HTML img element whose src is read from a string getter.
type Img struct{ bind.Getter[string] }

// NewImg returns an img widget whose src attribute is read from g.
func NewImg(g bind.Getter[string]) *Img { return &Img{Getter: g} }

// JawsRender renders ui as an HTML img element.
func (u *Img) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	var getterAttrs []template.HTMLAttr
	if _, getterAttrs, err = elem.ApplyGetter(u.Getter); err == nil {
		srcAttr := htmlio.Attr("src", u.JawsGet(elem))
		attrs := append(elem.ApplyParams(params), getterAttrs...)
		attrs = append(attrs, srcAttr)
		err = htmlio.WriteHTMLInner(w, elem.Jid(), "img", "", "", attrs...)
	}
	return
}

// JawsUpdate updates the src attribute.
//
// Like the other display widgets and unlike the typed inputs, Img keeps no
// last-rendered value and re-sends src on every update; mark the [jaws.Element]
// dirty only when src has actually changed.
func (u *Img) JawsUpdate(elem *jaws.Element) { elem.SetAttr("src", u.JawsGet(elem)) }

// Img renders an HTML img element.
func (rw RequestWriter) Img(imageSrc any, params ...any) error {
	return rw.NewUI(NewImg(bind.MakeGetter[string](imageSrc)), params...)
}
