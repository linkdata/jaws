package jaws

import (
	"html/template"
)

type UiSelect struct {
	UiInputSelect
}

func NewUiSelect(up Params) (ui *UiSelect) {
	ui = &UiSelect{
		UiInputSelect{
			UiInput:        NewUiInput(up),
			NamedBoolArray: up.nba,
		},
	}
	return
}

func (rq *Request) Select(namedBoolArray interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiSelect(NewParams(namedBoolArray, params)), params...)
}
