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
}

// UI creates an element for ui and renders it to the underlying writer.
func (rw RequestWriter) UI(ui jaws.UI, params ...any) (err error) {
	elem := rw.NewElement(ui)
	if err = elem.JawsRender(rw, params); err != nil {
		rw.DeleteElement(elem)
	}
	return
}

// Write records that rendering has started, then writes p to the underlying writer.
func (rw RequestWriter) Write(p []byte) (n int, err error) {
	rw.Rendering.Store(true)
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

// HeadHTML outputs the HTML code needed in the head section.
func (rw RequestWriter) HeadHTML() error {
	return rw.Request.HeadHTML(rw)
}

// TailHTML writes optional HTML code at the end of the page's BODY section that
// will immediately apply updates made during initial rendering.
func (rw RequestWriter) TailHTML() error {
	return rw.Request.TailHTML(rw)
}
