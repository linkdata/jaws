package jaws

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func TestJaws_Session(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	dot := Tag("123")

	h := rq.EnsureSession(rq.Handler("testtemplate", dot))
	var buf bytes.Buffer
	var rr httptest.ResponseRecorder
	rr.Body = &buf
	r := httptest.NewRequest("GET", "/", nil)

	if sess := rq.GetSession(r); sess != nil {
		t.Error("session already exists")
	}

	h.ServeHTTP(&rr, r)
	if got := buf.String(); got != `<div id="Jid.1" >123</div>` {
		t.Error(got)
	}

	sess := rq.GetSession(r)
	if sess == nil {
		t.Error("no session")
	}
}
