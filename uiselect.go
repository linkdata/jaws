package jaws

import (
	"html/template"
)

type UiSelect struct {
	UiInputSelect
}

func NewUiSelect(nba *NamedBoolArray) *UiSelect {
	return &UiSelect{
		UiInputSelect{
			NamedBoolArray: nba,
		},
	}
}

func (rq *Request) Select(nba *NamedBoolArray, params ...interface{}) template.HTML {
	return rq.UI(NewUiSelect(nba), params...)
}
