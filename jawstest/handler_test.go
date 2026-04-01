package jawstest

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/jtag"
	"github.com/linkdata/jaws/ui"
)

func TestHandler_ServeHTTP(t *testing.T) {
	jaws.NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	dot := jtag.Tag("123")
	h := ui.Handler(rq.TestRequest.Request.Jaws, "testtemplate", dot)
	var buf bytes.Buffer
	var rr httptest.ResponseRecorder
	rr.Body = &buf
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(&rr, r)
	if got := buf.String(); got != `<div id="Jid.1" >123</div>` {
		t.Error(got)
	}
}
