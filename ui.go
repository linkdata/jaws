package jaws

import (
	"time"

	pkg "github.com/linkdata/jaws/jaws"
)

// The point of this is to not have a zillion files in the repository root
// while keeping the import path unchanged.

type (
	UiA         = pkg.UiA
	UiButton    = pkg.UiButton
	UiCheckbox  = pkg.UiCheckbox
	UiContainer = pkg.UiContainer
	UiDate      = pkg.UiDate
	UiDiv       = pkg.UiDiv
	UiImg       = pkg.UiImg
	UiLabel     = pkg.UiLabel
	UiLi        = pkg.UiLi
	UiNumber    = pkg.UiNumber
	UiPassword  = pkg.UiPassword
	UiRadio     = pkg.UiRadio
	UiRange     = pkg.UiRange
	UiSelect    = pkg.UiSelect
	UiSpan      = pkg.UiSpan
	UiTbody     = pkg.UiTbody
	UiTd        = pkg.UiTd
	UiText      = pkg.UiText
	UiTr        = pkg.UiTr
)

func NewUiA(innerHTML HTMLGetter) *UiA {
	return pkg.NewUiA(innerHTML)
}
func NewUiButton(innerHTML HTMLGetter) *UiButton {
	return pkg.NewUiButton(innerHTML)
}
func NewUiCheckbox(g Setter[bool]) *UiCheckbox {
	return pkg.NewUiCheckbox(g)
}
func NewUiContainer(outerHTMLTag string, c Container) *UiContainer {
	return pkg.NewUiContainer(outerHTMLTag, c)
}
func NewUiDate(g Setter[time.Time]) *UiDate {
	return pkg.NewUiDate(g)
}
func NewUiDiv(innerHTML HTMLGetter) *UiDiv {
	return pkg.NewUiDiv(innerHTML)
}
func NewUiImg(g Getter[string]) *UiImg {
	return pkg.NewUiImg(g)
}
func NewUiLabel(innerHTML HTMLGetter) *UiLabel {
	return pkg.NewUiLabel(innerHTML)
}
func NewUiLi(innerHTML HTMLGetter) *UiLi {
	return pkg.NewUiLi(innerHTML)
}
func NewUiNumber(g Setter[float64]) *UiNumber {
	return pkg.NewUiNumber(g)
}
func NewUiPassword(g Setter[string]) *UiPassword {
	return pkg.NewUiPassword(g)
}
func NewUiRadio(vp Setter[bool]) *UiRadio {
	return pkg.NewUiRadio(vp)
}
func NewUiRange(g Setter[float64]) *UiRange {
	return pkg.NewUiRange(g)
}
func NewUiSelect(sh SelectHandler) *UiSelect {
	return pkg.NewUiSelect(sh)
}
func NewUiSpan(innerHTML HTMLGetter) *UiSpan {
	return pkg.NewUiSpan(innerHTML)
}
func NewUiTbody(c Container) *UiTbody {
	return pkg.NewUiTbody(c)
}
func NewUiTd(innerHTML HTMLGetter) *UiTd {
	return pkg.NewUiTd(innerHTML)
}
func NewUiText(vp Setter[string]) *UiText {
	return pkg.NewUiText(vp)
}
func NewUiTr(innerHTML HTMLGetter) *UiTr {
	return pkg.NewUiTr(innerHTML)
}
