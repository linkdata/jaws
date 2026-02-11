//go:build integration
// +build integration

package jawstest

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func TestHandler_ServeHTTP(t *testing.T) {
	nextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	dot := Tag("123")
	h := rq.Jaws.Handler("testtemplate", dot)
	var buf bytes.Buffer
	var rr httptest.ResponseRecorder
	rr.Body = &buf
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(&rr, r)
	if got := buf.String(); got != `<div id="Jid.1" >123</div>` {
		t.Error(got)
	}
}
