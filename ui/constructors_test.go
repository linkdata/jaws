package ui

import (
	"html/template"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/bind"
	"github.com/linkdata/jaws/jawsbool"
)

func TestConstructors(t *testing.T) {
	var mu sync.Mutex
	txt := ""
	checked := false
	num := 0.0
	when := time.Now()

	textSetter := bind.New(&mu, &txt)
	boolSetter := bind.New(&mu, &checked)
	numSetter := bind.New(&mu, &num)
	timeSetter := bind.New(&mu, &when)

	htmlGetter := bind.MakeHTMLGetter("x")
	imgGetter := bind.StringGetterFunc(func(*jaws.Element) string { return "img" })
	nba := jawsbool.NewNamedBoolArray(false).Add("a", template.HTML("A"))
	tc := testContainer{contents: []jaws.UI{NewSpan(htmlGetter)}}

	all := []jaws.UI{
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
