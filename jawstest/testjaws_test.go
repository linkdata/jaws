package jawstest

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

type testJaws struct {
	*Jaws
	testtmpl *template.Template
	log      bytes.Buffer
}

func newTestJaws() (tj *testJaws) {
	jw, err := New()
	if err != nil {
		panic(err)
	}
	tj = &testJaws{
		Jaws: jw,
	}
	tj.Jaws.Logger = slog.New(slog.NewTextHandler(&tj.log, nil))
	tj.Jaws.MakeAuth = func(r *Request) Auth {
		return DefaultAuth{}
	}
	tj.testtmpl = template.Must(template.New("testtemplate").Parse(`{{with $.Dot}}<div id="{{$.Jid}}" {{$.Attrs}}>{{.}}</div>{{end}}`))
	tj.AddTemplateLookuper(tj.testtmpl)

	tj.Jaws.updateTicker = time.NewTicker(time.Millisecond)
	go tj.Serve()
	return
}

func (tj *testJaws) newRequest(hr *http.Request) (tr *TestRequest) {
	return NewTestRequest(tj.Jaws, hr)
}

func newTestRequest(t *testing.T) (tr *TestRequest) {
	tj := newTestJaws()
	if t != nil {
		t.Helper()
		t.Cleanup(tj.Close)
	}
	return NewTestRequest(tj.Jaws, nil)
}
