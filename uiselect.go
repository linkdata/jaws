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

func (ui *UiSelect) JawsUpdate(u *Element) {
	nba := ui.UiContainer.Container.(*NamedBoolArray)
	u.SetValue(nba.Get())
	ui.UiContainer.JawsUpdate(u)
}

func (ui *UiSelect) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if wht == what.Input {
		nba := ui.UiContainer.Container.(*NamedBoolArray)
		nba.Set(val, true)
		e.Jaws.Dirty(ui.UiContainer.Container)
	}
	return
}

func (rq *Request) Select(nba Container, params ...interface{}) template.HTML {
	return rq.UI(NewUiSelect(nba), params...)
}
