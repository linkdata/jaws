package ui

import (
	"html/template"
	"sync"
	"testing"
	"time"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/jawsbind"
	"github.com/linkdata/jaws/core/jawsbool"
)

func TestConstructors(t *testing.T) {
	var mu sync.Mutex
	txt := ""
	checked := false
	num := 0.0
	when := time.Now()

	textSetter := jawsbind.Bind(&mu, &txt)
	boolSetter := jawsbind.Bind(&mu, &checked)
	numSetter := jawsbind.Bind(&mu, &num)
	timeSetter := jawsbind.Bind(&mu, &when)

	htmlGetter := jawsbind.MakeHTMLGetter("x")
	imgGetter := jawsbind.StringGetterFunc(func(*core.Element) string { return "img" })
	nba := jawsbool.NewNamedBoolArray(false).Add("a", template.HTML("A"))
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
