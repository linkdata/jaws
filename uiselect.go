package jaws

import (
	"html/template"

	"github.com/linkdata/jaws/what"
)

type UiSelect struct {
	UiContainer
}

func NewUiSelect(nba Container) *UiSelect {
	return &UiSelect{
		UiContainer{
			OuterHTMLTag: "select",
			Container:    nba,
		},
	}
}

func (ui *UiSelect) JawsUpdate(e *Element) {
	e.SetValue(ui.UiContainer.Container.(StringGetter).JawsGetString(e))
	ui.UiContainer.JawsUpdate(e)
}

func (ui *UiSelect) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if wht == what.Input {
		ui.UiContainer.Container.(StringSetter).JawsSetString(e, val)
		e.Jaws.Dirty(ui.UiContainer.Container)
	}
	return
}

func (rq *Request) Select(nba Container, params ...interface{}) template.HTML {
	return rq.UI(NewUiSelect(nba), params...)
}
