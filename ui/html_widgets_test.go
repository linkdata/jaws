package ui

import (
	"errors"
	"html/template"
	"strings"
	"testing"

	"github.com/linkdata/jaws/core"
)

func TestHTMLWidgets_ConstructorsAndRender(t *testing.T) {
	_, rq := newRequest(t)

	tests := []struct {
		name    string
		ui      core.UI
		params  []any
		pattern string
	}{
		{"A", NewA(testHTMLGetter("inner")), nil, `^<a id="Jid\.[0-9]+">inner</a>$`},
		{"Button", NewButton(testHTMLGetter("inner")), nil, `^<button id="Jid\.[0-9]+" type="button">inner</button>$`},
		{"Div", NewDiv(testHTMLGetter("inner")), nil, `^<div id="Jid\.[0-9]+">inner</div>$`},
		{"Label", NewLabel(testHTMLGetter("inner")), nil, `^<label id="Jid\.[0-9]+">inner</label>$`},
		{"Li", NewLi(testHTMLGetter("inner")), nil, `^<li id="Jid\.[0-9]+">inner</li>$`},
		{"Span", NewSpan(testHTMLGetter("inner")), nil, `^<span id="Jid\.[0-9]+">inner</span>$`},
		{"Td", NewTd(testHTMLGetter("inner")), nil, `^<td id="Jid\.[0-9]+">inner</td>$`},
		{"Tr", NewTr(testHTMLGetter("inner")), nil, `^<tr id="Jid\.[0-9]+">inner</tr>$`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elem, got := renderUI(t, rq, tt.ui, tt.params...)
			mustMatch(t, tt.pattern, got)
			tt.ui.JawsUpdate(elem)
		})
	}
}

func TestHTMLInner_RenderInnerApplyGetterError(t *testing.T) {
	_, rq := newRequest(t)

	wantErr := errors.New("init fail")
	g := &initFailGetter{err: wantErr}
	elem := rq.NewElement(NewA(g))
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); !errors.Is(err, wantErr) {
		t.Fatalf("want %v got %v", wantErr, err)
	}
}

type initFailGetter struct {
	err error
}

func (g *initFailGetter) JawsGetHTML(*core.Element) template.HTML { return "x" }
func (g *initFailGetter) JawsGetTag(*core.Request) any            { return g }
func (g *initFailGetter) JawsInit(*core.Element) error            { return g.err }

func TestImg_RenderAndUpdate(t *testing.T) {
	_, rq := newRequest(t)
	src := newTestSetter("image.png")
	ui := NewImg(src)
	elem, got := renderUI(t, rq, ui, "hidden")
	mustMatch(t, `^<img id="Jid\.[0-9]+" hidden src="image\.png">$`, got)
	src.Set("image2.jpg")
	ui.JawsUpdate(elem)
}

func TestOption_RenderAndUpdate(t *testing.T) {
	_, rq := newRequest(t)
	nba := core.NewNamedBoolArray()
	nb := core.NewNamedBool(nba, `escape"me`, "<unescaped>", true)
	ui := NewOption(nb)
	elem, got := renderUI(t, rq, ui, "hidden")
	mustMatch(t, `^<option id="Jid\.[0-9]+" hidden value="escape&#34;me" selected><unescaped></option>$`, got)

	nb.Set(false)
	ui.JawsUpdate(elem)
	nb.Set(true)
	ui.JawsUpdate(elem)
}

func TestRegister_Render(t *testing.T) {
	_, rq := newRequest(t)
	ui := NewRegister(NewSpan(testHTMLGetter("x")))
	_, got := renderUI(t, rq, ui)
	if got != "" {
		t.Fatalf("expected empty output got %q", got)
	}
}
