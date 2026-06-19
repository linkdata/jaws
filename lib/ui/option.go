package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/named"
)

// Option renders an HTML option element backed by a [named.Bool].
type Option struct{ *named.Bool }

// NewOption returns an option widget backed by nb.
func NewOption(nb *named.Bool) Option { return Option{Bool: nb} }

// JawsRender renders ui as an HTML option element.
func (u Option) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	// named.RenderBoolOption is the single source of <option> markup, so this cannot
	// diverge from the options a named.BoolArray renders.
	return named.RenderBoolOption(elem, w, u.Bool, params)
}

// JawsUpdate updates the selected attribute.
func (u Option) JawsUpdate(elem *jaws.Element) {
	named.UpdateBoolOption(elem, u.Bool)
}
