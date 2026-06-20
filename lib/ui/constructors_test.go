package ui

import (
	"html/template"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/named"
)

// TestConstructors is a smoke test that every exported widget constructor is
// wired up and returns a usable non-nil [jaws.UI]. The rendered output of each
// widget is pinned by the dedicated render tests (html_widgets_test.go,
// input_widgets_test.go and the per-widget tests), so this test deliberately
// asserts only non-nil rather than re-checking markup.
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
	nba := named.NewBoolArray(false).Add("a", template.HTML("A"))
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
