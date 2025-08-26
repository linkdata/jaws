package jaws

import (
	"io"
	"net/http"
)

type RequestWriter struct {
	rq *Request
	io.Writer
}

func (rw RequestWriter) UI(ui UI, params ...any) error {
	return rw.rq.NewElement(ui).JawsRender(rw, params)
}

func (rw RequestWriter) Write(p []byte) (n int, err error) {
	rw.rq.rendering.Store(true)
	return rw.Writer.Write(p)
}

// Request returns the current jaws.Request.
func (rw RequestWriter) Request() *Request {
	return rw.rq
}

// Initial returns the initial http.Request.
func (rw RequestWriter) Initial() *http.Request {
	return rw.Request().Initial()
}

// HeadHTML outputs the HTML code needed in the HEAD section.
func (rw RequestWriter) HeadHTML() error {
	return rw.Request().HeadHTML(rw)
}

// TailHTML writes optional HTML code at the end of the page's BODY section that
// will immediately apply updates made during initial rendering.
func (rw RequestWriter) TailHTML() error {
	return rw.Request().TailHTML(rw)
}

// Session returns the Requests's Session, or nil.
func (rw RequestWriter) Session() *Session {
	return rw.Request().Session()
}

// Get calls Request().Get()
func (rw RequestWriter) Get(key string) (val any) {
	return rw.Request().Get(key)
}

// Set calls Request().Set()
func (rw RequestWriter) Set(key string, val any) {
	rw.Request().Set(key, val)
}
