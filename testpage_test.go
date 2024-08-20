package jaws

import (
	"html/template"
	"io"
	"time"
)

const testPageTmplText = "(" +
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
	")"
const testPageNestedTmplText = "<x id=\"{{$.Jid}}\" {{$.Attrs}}>" +
	"{{$.Initial.URL.Path}}" +
	"{{with .Dot}}{{.}}{{$.Span `span2`}}{{end}}" +
	"</x>"

const testPageWant = "(" +
	"/" +
	"<a id=\"Jid.5\">a</a>" +
	"<button id=\"Jid.6\" type=\"button\">button</button>" +
	"<input id=\"Jid.7\" type=\"checkbox\" checkbox checked>" +
	"<container id=\"Jid.8\"></container>" +
	"<input id=\"Jid.9\" type=\"date\" value=\"1901-02-03\" dateattr>" +
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
	"<x id=\"Jid.22\" someattr>/dot<span id=\"Jid.23\">span2</span></x>" +
	"<input id=\"Jid.24\" type=\"text\" value=\"bar\">" +
	"<textarea id=\"Jid.25\">bar</textarea>" +
	"<tr id=\"Jid.26\">tr</tr>" +
	")"

type testPage struct {
	RequestWriter
	Normal       *template.Template
	TheBool      BoolSetter
	TheContainer Container
	TheTime      TimeSetter
	TheNumber    FloatSetter
	TheString    StringSetter
	TheSelector  SelectHandler
	TheDot       any
}

func newTestPage(tr *testRequest) *testPage {
	testDate, _ := time.Parse(ISO8601, "1901-02-03")
	tr.jw.AddTemplateLookuper(template.Must(template.New("nested").Parse(testPageNestedTmplText)))
	tmpl := template.Must(template.New("normal").Parse(testPageTmplText))

	tp := &testPage{
		RequestWriter: RequestWriter{rq: tr.rq},
		Normal:        tmpl,
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

	return tp
}

func (tp *testPage) render(w io.Writer) (err error) {
	nextJid = 4
	tp.RequestWriter.Writer = w
	return tp.Normal.Execute(w, tp)
}

func (tp *testPage) updateElems() {
	rq := tp.RequestWriter.rq
	for _, elem := range rq.elems {
		elem.JawsUpdate()
	}
}
