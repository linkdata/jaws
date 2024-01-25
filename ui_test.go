package jaws

import (
	"bytes"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type testStringer struct{}

func (s testStringer) String() string {
	return "I_Am_A_testStringer"
}

func TestRequest_JawsRender_DebugOutput(t *testing.T) {

	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	rq.Jaws.Debug = true
	rq.UI(&testUi{renderFn: func(e *Element, w io.Writer, params []any) error {
		e.Tag(Tag("footag"))
		e.Tag(e.Request)
		e.Tag(testStringer{})
		return nil
	}})
	h := rq.BodyString()
	t.Log(h)
	is.True(strings.Contains(h, "footag"))
	is.True(strings.Contains(h, "*jaws.testUi"))
	is.True(strings.Contains(h, testStringer{}.String()))
}

func TestRequest_InsideTemplate(t *testing.T) {
	jw := New()
	defer jw.Close()
	nextJid = 4

	const tmplText = "(" +
		"{{$.Initial.URL.Path}}" +
		"{{$.A `a`}}" +
		"{{$.Button `button`}}" +
		"{{$.Checkbox .TheBool `checkbox`}}" +
		"{{$.Container `container` .TheContainer}}" +
		"{{$.Date .TheTime `date`}}" +
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
		"{{$.Template `nested` .TheDot}}" +
		"{{$.Text .TheString}}" +
		"{{$.Textarea .TheString}}" +
		"{{$.Tr `tr`}}" +
		")"
	const nestedTmplText = "<x id=\"{{$.Jid}}\">" +
		"{{$.Initial.URL.Path}}" +
		"{{with .Dot}}{{.}}{{$.Span `span2`}}{{end}}" +
		"</x>"
	const want = "(" +
		"/path" +
		"<a id=\"Jid.5\">a</a>" +
		"<button id=\"Jid.6\" type=\"button\">button</button>" +
		"<input id=\"Jid.7\" type=\"checkbox\" checkbox checked>" +
		"<container id=\"Jid.8\"></container>" +
		"<input id=\"Jid.9\" type=\"date\" value=\"1901-02-03\" date>" +
		"<div id=\"Jid.10\">div</div>" +
		"<img id=\"Jid.11\" src=\"img\">" +
		"<label id=\"Jid.12\">label</label>" +
		"<li id=\"Jid.13\">li</li>" +
		"<input id=\"Jid.14\" type=\"number\" value=\"1.2\">" +
		"<input id=\"Jid.15\" type=\"password\" value=\"bar\">" +
		"<input id=\"Jid.16\" type=\"radio\" checked>" +
		"<input id=\"Jid.17\" type=\"range\" value=\"1.2\">" +
		"<select id=\"Jid.18\"></select>" +
		"<span id=\"Jid.19\">span</span>" +
		"<tbody id=\"Jid.20\"></tbody>" +
		"<td id=\"Jid.21\">td</td>" +
		"<x id=\"Jid.22\">/pathdot<span id=\"Jid.23\">span2</span></x>" +
		"<input id=\"Jid.24\" type=\"text\" value=\"bar\">" +
		"<textarea id=\"Jid.25\">bar</textarea>" +
		"<tr id=\"Jid.26\">tr</tr>" +
		")"

	jw.AddTemplateLookuper(template.Must(template.New("nested").Parse(nestedTmplText)))
	tmpl := template.Must(template.New("normal").Parse(tmplText))
	w := httptest.NewRecorder()
	w.Body = &bytes.Buffer{}
	hr := httptest.NewRequest(http.MethodGet, "/path", nil)
	rq := jw.NewRequest(hr)
	testDate, _ := time.Parse(ISO8601, "1901-02-03")
	dot := struct {
		RequestWriter
		TheBool      BoolSetter
		TheContainer Container
		TheTime      TimeSetter
		TheNumber    FloatSetter
		TheString    StringSetter
		TheSelector  SelectHandler
		TheDot       any
	}{
		RequestWriter: RequestWriter{rq, w},
		TheBool:       newTestSetter(true),
		TheContainer:  &testContainer{},
		TheTime:       newTestSetter(testDate),
		TheNumber:     newTestSetter(float64(1.2)),
		TheString:     newTestSetter("bar"),
		TheSelector: &testNamedBoolArray{
			setCalled:      make(chan struct{}),
			NamedBoolArray: NewNamedBoolArray(),
		},
		TheDot: "dot",
	}
	if err := tmpl.Execute(w, dot); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	if x := w.Body.String(); x != want {
		t.Errorf("%q", x)
	}
}
