package ui

import (
	"io"
	"net/http"

	"github.com/linkdata/jaws/core"
)

type RequestWriter struct {
	*core.Request
	io.Writer
}

func (rqw RequestWriter) UI(ui core.UI, params ...any) (err error) {
	elem := rqw.NewElement(ui)
	if err = elem.JawsRender(rqw, params); err != nil {
		rqw.DeleteElement(elem)
	}
	return
}

func (rqw RequestWriter) Write(p []byte) (n int, err error) {
	rqw.Rendering.Store(true)
	return rqw.Writer.Write(p)
}

// Initial returns the initial http.Request.
func (rqw RequestWriter) Initial() *http.Request {
	return rqw.Request.Initial()
}

// Session returns the Requests's Session, or nil.
func (rqw RequestWriter) Session() *core.Session {
	return rqw.Request.Session()
}

// Get calls Request().Get()
func (rqw RequestWriter) Get(key string) (val any) {
	return rqw.Request.Get(key)
}

// Set calls Request().Set()
func (rqw RequestWriter) Set(key string, val any) {
	rqw.Request.Set(key, val)
}

// HeadHTML outputs the HTML code needed in the HEAD section.
func (rqw RequestWriter) HeadHTML() error {
	return rqw.Request.HeadHTML(rqw)
}

// TailHTML writes optional HTML code at the end of the page's BODY section that
// will immediately apply updates made during initial rendering.
func (rqw RequestWriter) TailHTML() error {
	return rqw.Request.TailHTML(rqw)
}
