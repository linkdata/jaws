package ui

import (
	"html/template"
	"io"
	"strconv"

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
		srcAttr := template.HTMLAttr("src=" + strconv.Quote(u.JawsGet(elem))) // #nosec G203
		attrs := append(elem.ApplyParams(params), getterAttrs...)
		attrs = append(attrs, srcAttr)
		err = htmlio.WriteHTMLInner(w, elem.Jid(), "img", "", "", attrs...)
	}
	return
}

// JawsUpdate updates the src attribute.
func (u *Img) JawsUpdate(elem *jaws.Element) { elem.SetAttr("src", u.JawsGet(elem)) }

// Img renders an HTML img element.
func (rw RequestWriter) Img(imageSrc any, params ...any) error {
	return rw.UI(NewImg(bind.MakeGetter[string](imageSrc)), params...)
}
