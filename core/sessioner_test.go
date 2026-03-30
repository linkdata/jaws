package jaws

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func TestJaws_Session(t *testing.T) {
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	dot := Tag("123")

	h := rq.Jaws.Session(rq.Jaws.Handler("testtemplate", dot))
	var buf bytes.Buffer
	var rr httptest.ResponseRecorder
	rr.Body = &buf
	r := httptest.NewRequest("GET", "/", nil)

	if sess := rq.Jaws.GetSession(r); sess != nil {
		t.Error("session already exists")
	}

	h.ServeHTTP(&rr, r)
	if got := buf.String(); got != `<div id="Jid.1" >123</div>` {
		t.Error(got)
	}

	sess := rq.Jaws.GetSession(r)
	if sess == nil {
		t.Error("no session")
	}
}
