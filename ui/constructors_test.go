package ui

import (
	"html/template"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws/core"
)

func TestConstructors(t *testing.T) {
	var mu sync.Mutex
	txt := ""
	checked := false
	num := 0.0
	when := time.Now()

	textSetter := core.Bind(&mu, &txt)
	boolSetter := core.Bind(&mu, &checked)
	numSetter := core.Bind(&mu, &num)
	timeSetter := core.Bind(&mu, &when)

	htmlGetter := core.MakeHTMLGetter("x")
	imgGetter := core.StringGetterFunc(func(*core.Element) string { return "img" })
	nba := core.NewNamedBoolArray().Add("a", template.HTML("A"))
	tc := testContainer{contents: []core.UI{NewSpan(htmlGetter)}}

	all := []core.UI{
		NewA(htmlGetter),
		NewButton(htmlGetter),
		NewCheckbox(boolSetter),
		NewContainer("div", &tc),
		NewDate(timeSetter),
		NewDiv(htmlGetter),
		NewImg(imgGetter),
		NewLabel(htmlGetter),
		NewLi(htmlGetter),
		NewNumber(numSetter),
		NewPassword(textSetter),
		NewRadio(boolSetter),
		NewRange(numSetter),
		NewSelect(nba),
		NewSpan(htmlGetter),
		NewTbody(&tc),
		NewTd(htmlGetter),
		NewText(textSetter),
		NewTextarea(textSetter),
		NewTr(htmlGetter),
	}
	for i, ui := range all {
		if ui == nil {
			t.Fatalf("constructor[%d] returned nil", i)
		}
	}
}
