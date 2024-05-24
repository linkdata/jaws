package jaws

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func TestTemplate_String(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	dot := 123
	tmpl := rq.Jaws.NewTemplate("testtemplate", dot)

	is.Equal(tmpl.String(), `{"testtemplate", 123}`)
}

func TestTemplate_Calls_Dot_Updater(t *testing.T) {
	rq := newTestRequest()
	defer rq.Close()

	dot := &testUi{}
	tmpl := rq.Jaws.NewTemplate("testtemplate", dot)
	tmpl.JawsUpdate(nil)
	if dot.updateCalled != 1 {
		t.Error(dot.updateCalled)
	}
}

func TestTemplate_As_Handler(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	dot := 123
	tmpl := rq.Jaws.NewTemplate("testtemplate", dot)
	var buf bytes.Buffer
	var rr httptest.ResponseRecorder
	rr.Body = &buf
	r := httptest.NewRequest("GET", "/", nil)
	tmpl.ServeHTTP(&rr, r)
	if got := buf.String(); got != `<div id="Jid.1" >123</div>` {
		t.Error(got)
	}
}
