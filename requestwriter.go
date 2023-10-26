package jaws

import "io"

type RequestWriter struct {
	*Request
	io.Writer
}

func (rw RequestWriter) UI(ui UI, params ...interface{}) error {
	return rw.JawsRender(rw.NewElement(ui), rw.Writer, params)
}

type ElementWriter struct {
	*Element
	RequestWriter
}
