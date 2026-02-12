package jawstest

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"testing"

	"github.com/linkdata/jaws"
)

type testJaws struct {
	*jaws.Jaws
	testtmpl *template.Template
	log      bytes.Buffer
}

func newTestJaws() (tj *testJaws) {
	jw, err := jaws.New()
	if err != nil {
		panic(err)
	}
	tj = &testJaws{
		Jaws: jw,
	}
	tj.Jaws.Logger = slog.New(slog.NewTextHandler(&tj.log, nil))
	tj.Jaws.MakeAuth = func(r *jaws.Request) jaws.Auth {
		return testAuth{}
	}
	tj.testtmpl = template.Must(template.New("testtemplate").Parse(`{{with $.Dot}}<div id="{{$.Jid}}" {{$.Attrs}}>{{.}}</div>{{end}}`))
	tj.AddTemplateLookuper(tj.testtmpl)
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

type testAuth struct{}

func (testAuth) Data() map[string]any { return nil }
func (testAuth) Email() string        { return "" }
func (testAuth) IsAdmin() bool        { return true }
