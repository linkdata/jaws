package jaws

import (
	"html/template"
)

type UiSelect struct {
	UiInputSelect
}

func MakeUiSelect(nba *NamedBoolArray) UiSelect {
	return UiSelect{
		UiInputSelect{
			NamedBoolArray: nba,
		},
	}
}

func (rq *Request) Select(nba *NamedBoolArray, params ...interface{}) template.HTML {
	ui := MakeUiSelect(nba)
	return rq.UI(&ui, params...)
}
