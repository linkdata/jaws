package ui

import (
	"html/template"
	"sync"
	"testing"
	"time"

	pkg "github.com/linkdata/jaws/jaws"
)

func TestConstructors(t *testing.T) {
	var mu sync.Mutex
	txt := ""
	checked := false
	num := 0.0
	when := time.Now()

	textSetter := pkg.Bind(&mu, &txt)
	boolSetter := pkg.Bind(&mu, &checked)
	numSetter := pkg.Bind(&mu, &num)
	timeSetter := pkg.Bind(&mu, &when)

	htmlGetter := pkg.MakeHTMLGetter("x")
	imgGetter := pkg.StringGetterFunc(func(*pkg.Element) string { return "img" })
	nba := pkg.NewNamedBoolArray().Add("a", template.HTML("A"))
	tc := testContainer{contents: []pkg.UI{NewSpan(htmlGetter)}}

	all := []pkg.UI{
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
