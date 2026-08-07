package ui

import (
	"io"
	"net/http"

	"github.com/linkdata/jaws"
)

// RequestWriter combines a [jaws.Request] with an [io.Writer] while rendering.
type RequestWriter struct {
	*jaws.Request
	io.Writer
	// elementCreated, when non-nil, is called with every Element created through
	// this writer, letting the widget that owns the writer track them. It is called
	// as soon as the Element exists, before it renders and whether or not it ever
	// does, so an implementation must not assume rendered state.
	//
	// It is deliberately unexported: RequestWriter is embedded in [With], so an
	// exported func field would be callable from any template as
	// {{call $.ElementCreated $.Element}}, letting a template make an Element own
	// itself and have its own wrapper unregistered on the next update.
	elementCreated func(elem *jaws.Element)
}

// trackElement reports a newly created Element to the writer's owner, if any.
func (rw RequestWriter) trackElement(elem *jaws.Element) {
	if rw.elementCreated != nil {
		rw.elementCreated(elem)
	}
}

// NewUI creates an element for ui and renders it to the underlying writer.
//
// The ui value must satisfy the ownership and live-Element multiplicity
// requirements documented by [jaws.UI].
func (rw RequestWriter) NewUI(ui jaws.UI, params ...any) (err error) {
	elem := rw.NewElement(ui)
	// Report the Element before rendering it, so the owner's set is complete even for
	// one that fails. That set may then hold an Element already unregistered below,
	// which costs nothing: Request.DeleteElements skips elements it finds
	// unregistered, and every rollback path deletes the whole set at once.
	rw.trackElement(elem)
	if err = elem.JawsRender(rw, params); err != nil {
		// Unregister anything the failed Element already owns along with it, so no
		// widget can strand a subtree by not rolling back itself.
		deleteOwnedElements(rw.Request, []*jaws.Element{elem})
	}
	return
}

// Write marks render activity through [jaws.Request.MarkWritten] before writing p
// to the underlying writer.
func (rw RequestWriter) Write(p []byte) (n int, err error) {
	rw.MarkWritten()
	return rw.Writer.Write(p)
}

// Initial returns the initial [http.Request].
func (rw RequestWriter) Initial() *http.Request {
	return rw.Request.Initial()
}

// Session returns the request's [jaws.Session], or nil.
func (rw RequestWriter) Session() *jaws.Session {
	return rw.Request.Session()
}

// Get calls [jaws.Request.Get].
func (rw RequestWriter) Get(key string) (value any) {
	return rw.Request.Get(key)
}

// Set calls [jaws.Request.Set].
func (rw RequestWriter) Set(key string, value any) {
	rw.Request.Set(key, value)
}

// HeadHTML calls [jaws.Request.HeadHTML].
func (rw RequestWriter) HeadHTML() error {
	return rw.Request.HeadHTML(rw)
}

// TailHTML writes optional HTML code at the end of the page's BODY section that
// will immediately apply updates made during initial rendering.
func (rw RequestWriter) TailHTML() error {
	return rw.Request.TailHTML(rw)
}
