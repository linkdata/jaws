package jaws

import (
	"html/template"
)

type UiSelect struct {
	UiInputSelect
}

func (rq *Request) Select(nba *NamedBoolArray, fn InputTextFn, attrs ...interface{}) template.HTML {
	ui := &UiSelect{
		UiInputSelect{
			UiHtml:         UiHtml{Tags: StringTags(nba.Tagstring)},
			NamedBoolArray: nba,
			InputTextFn:    fn,
		},
	}
	return rq.UI(ui, attrs...)
}
