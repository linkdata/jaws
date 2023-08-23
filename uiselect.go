package jaws

import (
	"html/template"
)

type UiSelect struct {
	UiInputSelect
}

func NewUiSelect(tags []interface{}, nba *NamedBoolArray) (ui *UiSelect) {
	ui = &UiSelect{
		UiInputSelect{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: append(tags, nba)},
			},
			NamedBoolArray: nba,
		},
	}
	return
}

func (rq *Request) Select(tagitem interface{}, nba *NamedBoolArray, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiSelect(ProcessTags(tagitem), nba), attrs...)
}
