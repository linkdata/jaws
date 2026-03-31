package ui

import (
	"html/template"
	"sync"
	"testing"
	"time"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/bind"
	"github.com/linkdata/jaws/core/named"
)

func TestConstructors(t *testing.T) {
	var mu sync.Mutex
	txt := ""
	checked := false
	num := 0.0
	when := time.Now()

	textSetter := bind.Bind(&mu, &txt)
	boolSetter := bind.Bind(&mu, &checked)
	numSetter := bind.Bind(&mu, &num)
	timeSetter := bind.Bind(&mu, &when)

	htmlGetter := bind.MakeHTMLGetter("x")
	imgGetter := bind.StringGetterFunc(func(*core.Element) string { return "img" })
	nba := named.NewNamedBoolArray(false).Add("a", template.HTML("A"))
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
