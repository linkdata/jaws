package jaws_test

// this is just to satisfy coverage,
// proper tests are in jaws/jaws

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws"
)

const testPageTmplText = "({{with .Dot}}" +
	"{{$.Initial.URL.Path}}" +
	"{{$.A `a`}}" +
	"{{$.Button `button`}}" +
	"{{$.Checkbox .TheBool `checkbox`}}" +
	"{{$.Container `container` .TheContainer}}" +
	"{{$.Date .TheTime `dateattr`}}" +
	"{{$.Div `div`}}" +
	"{{$.Img `img`}}" +
	"{{$.Label `label`}}" +
	"{{$.Li `li`}}" +
	"{{$.Number .TheNumber}}" +
	"{{$.Password .TheString}}" +
	"{{$.Radio .TheBool}}" +
	"{{$.Range .TheNumber}}" +
	"{{$.Select .TheSelector}}" +
	"{{$.Span `span`}}" +
	"{{$.Tbody .TheContainer}}" +
	"{{$.Td `td`}}" +
	"{{$.Template `nested` .TheDot `someattr`}}" +
	"{{$.Text .TheString}}" +
	"{{$.Textarea .TheString}}" +
	"{{$.Tr `tr`}}" +
	"{{end}})"
const testPageNestedTmplText = "<x id=\"{{$.Jid}}\" {{$.Attrs}}>" +
	"{{$.Initial.URL.Path}}" +
	"{{with .Dot}}{{.}}{{$.Span `span2`}}{{end}}" +
	"</x>"

const testPageWant = "(" +
	"/" +
	"<a id=\"Jid.2\">a</a>" +
	"<button id=\"Jid.3\" type=\"button\">button</button>" +
	"<input id=\"Jid.4\" type=\"checkbox\" checkbox checked>" +
	"<container id=\"Jid.5\"></container>" +
	"<input id=\"Jid.6\" type=\"date\" value=\"1901-02-03\" dateattr>" +
	"<div id=\"Jid.7\">div</div>" +
	"<img id=\"Jid.8\" src=\"img\">" +
	"<label id=\"Jid.9\">label</label>" +
	"<li id=\"Jid.10\">li</li>" +
	"<input id=\"Jid.11\" type=\"number\" value=\"1.2\">" +
	"<input id=\"Jid.12\" type=\"password\" value=\"bar\">" +
	"<input id=\"Jid.13\" type=\"radio\" checked>" +
	"<input id=\"Jid.14\" type=\"range\" value=\"1.2\">" +
	"<select id=\"Jid.15\"></select>" +
	"<span id=\"Jid.16\">span</span>" +
	"<tbody id=\"Jid.17\"></tbody>" +
	"<td id=\"Jid.18\">td</td>" +
	"<x id=\"Jid.19\" someattr>/dot<span id=\"Jid.20\">span2</span></x>" +
	"<input id=\"Jid.21\" type=\"text\" value=\"bar\">" +
	"<textarea id=\"Jid.22\">bar</textarea>" +
	"<tr id=\"Jid.23\">tr</tr>" +
	")"

type testContainer struct{ contents []jaws.UI }

func (tc *testContainer) JawsContains(e *jaws.Element) (contents []jaws.UI) {
	return tc.contents
}

type testPage struct {
	jaws.RequestWriter
	TheBool      jaws.Setter[bool]
	TheContainer jaws.Container
	TheTime      jaws.Setter[time.Time]
	TheNumber    jaws.Setter[float64]
	TheString    jaws.Setter[string]
	TheSelector  jaws.SelectHandler
	TheDot       any
}

func maybeFatal(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewTemplate(t *testing.T) {
	jw, err := jaws.New()
	maybeFatal(t, err)
	defer jw.Close()

	jw.AddTemplateLookuper(template.Must(template.New("nested").Parse(testPageNestedTmplText)))
	jw.AddTemplateLookuper(template.Must(template.New("normal").Parse(testPageTmplText)))

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	jw.UseRequest(rq.JawsKey, hr)
	var sb strings.Builder
	rqwr := rq.Writer(&sb)

	var mu sync.RWMutex
	vbool := true
	vtime, _ := time.Parse(jaws.ISO8601, "1901-02-03")
	vnumber := float64(1.2)
	vstring := "bar"
	nba := jaws.NewNamedBoolArray()

	tp := &testPage{
		TheBool:      jaws.Bind(&mu, &vbool),
		TheContainer: &testContainer{},
		TheTime:      jaws.Bind(&mu, &vtime),
		TheNumber:    jaws.Bind(&mu, &vnumber),
		TheString:    jaws.Bind(&mu, &vstring),
		TheSelector:  nba,
		TheDot:       jaws.Tag("dot"),
	}

	tmpl := jaws.NewTemplate("normal", tp)
	elem := rq.NewElement(tmpl)
	err = tmpl.JawsRender(elem, rqwr, nil)
	maybeFatal(t, err)

	if sb.String() != testPageWant {
		t.Errorf("\n got: %q\nwant: %q\n", sb.String(), testPageWant)
	}
}

func TestJsVar(t *testing.T) {
	var mu sync.RWMutex
	vbool := true
	_ = jaws.NewJsVar(&mu, &vbool)
}

func TestJawsKeyString(t *testing.T) {
	if s := jaws.JawsKeyString(1000); s != "v8" {
		t.Error(s)
	}
}

func TestWriteHTMLTag(t *testing.T) {
	var sb strings.Builder
	err := jaws.WriteHTMLTag(&sb, 1, "foo", "checkbox", "false", []template.HTMLAttr{"someattr"})
	maybeFatal(t, err)
	want := "<foo id=\"Jid.1\" type=\"checkbox\" value=\"false\" someattr>"
	if sb.String() != want {
		t.Errorf("\n got: %q\nwant: %q\n", sb.String(), want)
	}
}

func TestNewUi(t *testing.T) {
	htmlGetter := jaws.MakeHTMLGetter("x")
	htmlGetter2 := jaws.HTMLGetterFunc(func(elem *jaws.Element) (tmpl template.HTML) {
		return "x"
	})
	stringGetter := jaws.StringGetterFunc(func(elem *jaws.Element) (s string) {
		return "s"
	})
	var mu sync.RWMutex
	vbool := true
	vtime, _ := time.Parse(jaws.ISO8601, "1901-02-03")
	vnumber := float64(1.2)
	vstring := "bar"
	nba := jaws.NewNamedBoolArray()
	_ = jaws.NewNamedBool(nba, "escape\"me", "<unescaped>", true)

	jaws.NewUiA(htmlGetter)
	jaws.NewUiButton(htmlGetter2)
	jaws.NewUiCheckbox(jaws.Bind(&mu, &vbool))
	jaws.NewUiContainer("tbody", &testContainer{})
	jaws.NewUiDate(jaws.Bind(&mu, &vtime))
	jaws.NewUiDiv(htmlGetter)
	jaws.NewUiImg(stringGetter)
	jaws.NewUiLabel(htmlGetter)
	jaws.NewUiLi(htmlGetter)
	jaws.NewUiNumber(jaws.Bind(&mu, &vnumber))
	jaws.NewUiPassword(jaws.Bind(&mu, &vstring))
	jaws.NewUiRadio(jaws.Bind(&mu, &vbool))
	jaws.NewUiRange(jaws.Bind(&mu, &vnumber))
	jaws.NewUiSelect(nba)
	jaws.NewUiSpan(htmlGetter)
	jaws.NewUiTbody(&testContainer{})
	jaws.NewUiTd(htmlGetter)
	jaws.NewUiText(jaws.Bind(&mu, &vstring))
	jaws.NewUiTr(htmlGetter)

}
