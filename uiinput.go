package jaws

import (
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
		ui.ValueGetter = &defaultValueHandler{v: data}
	case bool:
		ui.ValueGetter = &defaultValueHandler{v: data}
	case time.Time:
		ui.ValueGetter = &defaultValueHandler{v: data}
	case int:
		ui.ValueGetter = &defaultValueHandler{v: float64(data)}
	case float32:
		ui.ValueGetter = &defaultValueHandler{v: float64(data)}
	case float64:
		ui.ValueGetter = &defaultValueHandler{v: data}
	}
}

func (ui *UiInput) Get() interface{} {
	if ui.ValueGetter != nil {
		return ui.ValueGetter.JawsGet()
	}
	panic("jaws: UiInput: no ValueGetter")
}

func (ui *UiInput) Set(value interface{}) error {
	if ui.ValueSetter != nil {
		return ui.ValueSetter.JawsSet(value)
	}
	panic("jaws: UiInput: no ValueSetter")
}
