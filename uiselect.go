package jaws

import (
	"html/template"
)

type UiSelect struct {
	UiInputSelect
}

func (rq *Request) Select(nba *NamedBoolArray, attrs ...interface{}) template.HTML {
	ui := &UiSelect{
		UiInputSelect{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: StringTags(nba.Tagstring)},
			},
			NamedBoolArray: nba,
		},
	}
	return rq.UI(ui, attrs...)
}
