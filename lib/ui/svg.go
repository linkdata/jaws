package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// SVG renders an SVG element with dynamic inner SVG markup.
type SVG struct{ HTMLInner }

// NewSVG returns an SVG widget whose inner markup is rendered from innerHTML.
// innerHTML is passed to [bind.MakeHTMLGetter]; plain strings are trusted markup.
func NewSVG(innerHTML any) *SVG {
	return &SVG{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}}
}

// JawsRender renders ui as an SVG element.
func (u *SVG) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderInner(elem, w, "svg", "", params)
}

// SVG renders an SVG element.
func (rw RequestWriter) SVG(innerHTML any, params ...any) error {
	return rw.UI(NewSVG(innerHTML), params...)
}

// SVGContainer renders an SVG element around a dynamic child collection.
type SVGContainer struct {
	ContainerHelper
}

// NewSVGContainer returns an SVG widget that renders and updates c as SVG child elements.
func NewSVGContainer(c jaws.Container) *SVGContainer {
	return &SVGContainer{ContainerHelper: NewContainerHelper(c)}
}

// JawsRender renders ui as an SVG element.
func (u *SVGContainer) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.RenderContainer(elem, w, "svg", params)
}

// JawsUpdate updates the SVG child collection.
func (u *SVGContainer) JawsUpdate(elem *jaws.Element) {
	u.UpdateContainer(elem)
}

// SVGContainer renders an SVG element around a dynamic child collection.
func (rw RequestWriter) SVGContainer(c jaws.Container, params ...any) error {
	return rw.UI(NewSVGContainer(c), params...)
}
