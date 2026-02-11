package jaws

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws/what"
)

type testDivWidget struct {
	inner template.HTML
}

func (ui testDivWidget) JawsRender(e *Element, w io.Writer, params []any) error {
	e.ApplyParams(params)
	return WriteHTMLInner(w, e.Jid(), "div", "", ui.inner)
}

func (testDivWidget) JawsUpdate(*Element) {}

type testTextInputWidget struct {
	Setter[string]
	tag  any
	last string
}

func newTestTextInputWidget(s Setter[string]) *testTextInputWidget {
	return &testTextInputWidget{Setter: s}
}

func (ui *testTextInputWidget) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	if ui.tag, err = e.ApplyGetter(ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		v := ui.JawsGet(e)
		ui.last = v
		err = WriteHTMLInput(w, e.Jid(), "text", v, attrs)
	}
	return
}

func (ui *testTextInputWidget) JawsUpdate(e *Element) {
	if v := ui.JawsGet(e); v != ui.last {
		ui.last = v
		e.SetValue(v)
	}
}

func (ui *testTextInputWidget) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Input {
		if changed, setErr := e.maybeDirty(ui.tag, ui.Setter.JawsSet(e, val)); setErr != nil {
			err = setErr
		} else {
			err = nil
			if changed {
				ui.last = val
			}
		}
	}
	return
}
