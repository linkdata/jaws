package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/tag"
)

// Register is an update-only widget that renders no HTML; it exists so its
// embedded [jaws.Updater] receives dynamic updates.
type Register struct{ jaws.Updater }

// NewRegister returns an update-only widget that invokes updater during updates.
func NewRegister(updater jaws.Updater) Register { return Register{Updater: updater} }

type registerDirtyTagger interface {
	registerDirtyTag(*jaws.Element)
}

func registerGetterTag(elem *jaws.Element, getter any) (tagValue any) {
	tagValue = getter
	if tagger, ok := getter.(tag.TagGetter); ok {
		tagValue = tagger.JawsGetTag(elem.Request)
	}
	switch tagValue.(type) {
	case tag.TagGetter, []any, []tag.Tag:
		elem.Tag(tagValue)
	default:
		if tag.NewErrNotComparable(tagValue) == nil {
			elem.Tag(tagValue)
		}
	}
	return
}

func (u *InputText) registerDirtyTag(elem *jaws.Element) {
	u.tag = registerGetterTag(elem, u.Setter)
}

func (u *InputBool) registerDirtyTag(elem *jaws.Element) {
	u.tag = registerGetterTag(elem, u.Setter)
}

func (u *InputFloat) registerDirtyTag(elem *jaws.Element) {
	u.tag = registerGetterTag(elem, u.Setter)
}

func (u *InputDate) registerDirtyTag(elem *jaws.Element) {
	u.tag = registerGetterTag(elem, u.Setter)
}

func (u *Select) registerDirtyTag(elem *jaws.Element) {
	u.tag = registerGetterTag(elem, u.Container)
}

// JawsRender renders no HTML for update-only registration.
//
// It ignores params; to attach extra tags or event handlers, use
// [RequestWriter.Register], which applies them before the element is frozen.
func (u Register) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return nil
}

// Register creates a new Element with the given Updater as a tag
// for dynamic updates. Additional tags may be provided in params.
// If updater also implements an event handler interface, it receives matching
// events after handlers provided in params have had a chance to handle them.
// The updater's [jaws.Updater.JawsUpdate] method will be called immediately to
// ensure the initial rendering is correct. Standard input widgets also register
// their resolved backing tag, when usable, so an input event dirties every bound
// view; Register does not call their [jaws.Renderer.JawsRender] method.
//
// Returns a [jid.Jid], suitable for including as an HTML id attribute:
//
//	<div id="{{$.Register .MyUpdater}}">...</div>
func (rw RequestWriter) Register(updater jaws.Updater, params ...any) jid.Jid {
	elem := rw.NewElement(Register{Updater: updater})
	elem.Tag(updater)
	// The wrapping Register element's UI is not the updater, so events reach the
	// updater only through the element's handler list, not the elem.UI() fallback.
	switch updater.(type) {
	case jaws.InputHandler, jaws.ClickHandler, jaws.ContextMenuHandler:
		elem.AddHandlers(updater)
	}
	elem.ApplyParams(params)
	if tagger, ok := updater.(registerDirtyTagger); ok {
		tagger.registerDirtyTag(elem)
	}
	updater.JawsUpdate(elem)
	elem.Freeze()
	return elem.Jid()
}
