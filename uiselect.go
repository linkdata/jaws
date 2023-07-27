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
			UiHtml:         UiHtml{Tags: StringTags(nba.Tagstring)},
			NamedBoolArray: nba,
		},
	}
	return rq.UI(ui, attrs...)
}
