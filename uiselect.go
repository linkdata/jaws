package jaws

import (
	"html/template"
)

type UiSelect struct {
	UiInputSelect
}

func NewUiSelect(nba *NamedBoolArray) (ui *UiSelect) {
	ui = &UiSelect{
		UiInputSelect{
			UiInput: UiInput{
				UiValueProxy{
					ValueProxy: nba,
				},
			},
			NamedBoolArray: nba,
		},
	}
	return
}

func (rq *Request) Select(nba *NamedBoolArray, params ...interface{}) template.HTML {
	return rq.UI(NewUiSelect(nba), params...)
}
