package jaws

import (
	"io"
	"net/http"
)

type RequestWriter struct {
	rq *Request
	io.Writer
}

func (rw RequestWriter) UI(ui UI, params ...interface{}) error {
	return rw.rq.JawsRender(rw.rq.NewElement(ui), rw.Writer, params)
}

// Request returns the current jaws.Request.
func (rw RequestWriter) Request() *Request {
	return rw.rq
}

// Initial returns the initial http.Request.
func (rw RequestWriter) Initial() *http.Request {
	return rw.Request().Initial
}

// HeadHTML outputs the HTML code needed in the HEAD section.
func (rw RequestWriter) HeadHTML() error {
	return rw.Request().HeadHTML(rw)
}

// Session returns the Requests's Session, or nil.
func (rw RequestWriter) Session() *Session {
	return rw.Request().Session()
}

// Get calls Request().Get()
func (rw RequestWriter) Get(key string) (val interface{}) {
	return rw.Request().Get(key)
}

// Set calls Request().Set()
func (rw RequestWriter) Set(key string, val interface{}) {
	rw.Request().Set(key, val)
}
