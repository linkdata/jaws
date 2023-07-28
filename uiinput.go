package jaws

import (
	"html/template"
	"time"
)

type UiInput struct {
	UiHtml
	ValueProxy
}

func (ui *UiInput) ProcessValue(value interface{}) {
	switch data := value.(type) {
	case ValueProxy:
		ui.ValueProxy = data
	case template.HTML:
		ui.ValueProxy = &defaultValueProxy{v: data}
	case string:
		ui.ValueProxy = &defaultValueProxy{v: data}
	case bool:
		ui.ValueProxy = &defaultValueProxy{v: data}
	case time.Time:
		ui.ValueProxy = &defaultValueProxy{v: data}
	case int:
		ui.ValueProxy = &defaultValueProxy{v: float64(data)}
	case float32:
		ui.ValueProxy = &defaultValueProxy{v: float64(data)}
	case float64:
		ui.ValueProxy = &defaultValueProxy{v: data}
	}
	if ui.ValueProxy == nil {
		panic("jaws: UiInput: no ValueProxy")
	}
}

func (ui *UiInput) Get(e *Element) interface{} {
	return ui.ValueProxy.JawsGet(e)
}

func (ui *UiInput) Set(e *Element, value interface{}) (err error) {
	return ui.ValueProxy.JawsSet(e, value)
}
