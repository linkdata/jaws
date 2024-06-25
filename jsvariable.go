package jaws

import (
	"errors"
	"fmt"
	"html/template"
	"io"
)

var ErrMissingJavascriptName = errors.New("missing Javascript name")

type JsVariable struct {
	Name string
}

func (ui *JsVariable) render(getter any, e *Element, w io.Writer, params []any) error {
	e.ApplyGetter(getter)
	attrs := e.ApplyParams(params)
	innerHTML := template.HTML(fmt.Sprintf("var %s = null;\n", ui.Name)) //#nosec G203
	return WriteHtmlInner(w, e.Jid(), "script", "", innerHTML, attrs...)
}
