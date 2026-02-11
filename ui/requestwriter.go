package ui

import (
	"io"

	"github.com/linkdata/jaws/jaws"
)

type RequestWriter struct {
	*jaws.Request
	io.Writer
}

func (rqw RequestWriter) UI(ui jaws.UI, params ...any) error {
	return rqw.NewElement(ui).JawsRender(rqw, params)
}

func (rqw RequestWriter) Write(p []byte) (n int, err error) {
	rqw.Rendering.Store(true)
	return rqw.Writer.Write(p)
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
