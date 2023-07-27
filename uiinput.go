package jaws

import (
	"io"
	"time"
)

type UiInput struct {
	UiHtml
	ValueGetter
	ValueSetter
}

func (ui *UiInput) ProcessValue(value interface{}) {
	switch data := value.(type) {
	case ValueHandler:
		ui.ValueGetter = data
		ui.ValueSetter = data
	case ValueGetter:
		ui.ValueGetter = data
	case ValueSetter:
		ui.ValueSetter = data
	case string:
		ui.ValueGetter = &defaultValueHandler{Value: data}
	case bool:
		ui.ValueGetter = &defaultValueHandler{Value: data}
	case time.Time:
		ui.ValueGetter = &defaultValueHandler{Value: data}
	case float32:
		ui.ValueGetter = &defaultValueHandler{Value: float64(data)}
	case float64:
		ui.ValueGetter = &defaultValueHandler{Value: data}
	}
}

func (ui *UiInput) Get() (value interface{}) {
	if ui.ValueGetter != nil {
		return ui.ValueGetter.JawsGet()
	}
	panic("jaws: UiInput: no ValueGetter")
}

func (ui *UiInput) Set(value interface{}) (err error) {
	if ui.ValueSetter != nil {
		err = ui.ValueSetter.JawsSet(value)
	}
	return
}

func (ui *UiInput) WriteHtmlInput(rq *Request, w io.Writer, htmltype, htmlval, jid string, data ...interface{}) error {
	return WriteHtmlInput(w, jid, htmltype, htmlval, ui.UiHtml.ProcessData(data)...)
}
