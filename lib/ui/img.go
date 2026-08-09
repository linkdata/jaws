package ui

import (
	"io"
	"slices"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
)

// Img renders an HTML img element whose src is read from a string getter.
//
// The getter-derived src takes precedence over any src attribute passed as a
// render param or returned by the getter's [jaws.InitialHTMLAttrHandler].
//
// One Img value may back multiple live [jaws.Element] values. Its getter is
// shared by those Elements and must be safe for their render, update and event
// calls.
type Img struct{ bind.Getter[string] }

// NewImg returns an img widget whose src attribute is read from g.
func NewImg(g bind.Getter[string]) *Img { return &Img{Getter: g} }

// JawsRender renders ui as an HTML img element.
func (u *Img) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	elem.ApplyGetter(u.Getter)
	getterAttrs := elem.ApplyInitialHTMLAttr(u.Getter)
	srcAttr := htmlio.Attr("src", u.JawsGet(elem))
	attrs := append(elem.ApplyParams(params), getterAttrs...)
	// HTML parsing keeps the first duplicate attribute, so emit the canonical
	// src before caller and getter attributes.
	attrs = slices.Insert(attrs, 0, srcAttr)
	err = htmlio.WriteHTMLInner(w, elem.Jid(), "img", "", "", attrs...)
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
