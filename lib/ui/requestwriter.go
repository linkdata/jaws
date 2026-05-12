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
func (rqw RequestWriter) UI(ui jaws.UI, params ...any) (err error) {
	elem := rqw.NewElement(ui)
	if err = elem.JawsRender(rqw, params); err != nil {
		rqw.DeleteElement(elem)
	}
	return
}

// Write records that rendering has started, then writes p to the underlying writer.
func (rqw RequestWriter) Write(p []byte) (n int, err error) {
	rqw.Rendering.Store(true)
	return rqw.Writer.Write(p)
}

// Initial returns the initial [http.Request].
func (rqw RequestWriter) Initial() *http.Request {
	return rqw.Request.Initial()
}

// Session returns the request's [jaws.Session], or nil.
func (rqw RequestWriter) Session() *jaws.Session {
	return rqw.Request.Session()
}

// Get calls [jaws.Request.Get].
func (rqw RequestWriter) Get(key string) (val any) {
	return rqw.Request.Get(key)
}

// Set calls [jaws.Request.Set].
func (rqw RequestWriter) Set(key string, val any) {
	rqw.Request.Set(key, val)
}

// HeadHTML outputs the HTML code needed in the head section.
func (rqw RequestWriter) HeadHTML() error {
	return rqw.Request.HeadHTML(rqw)
}

// TailHTML writes optional HTML code at the end of the page's BODY section that
// will immediately apply updates made during initial rendering.
func (rqw RequestWriter) TailHTML() error {
	return rqw.Request.TailHTML(rqw)
}
