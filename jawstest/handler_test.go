package jawstest

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws"
	pkg "github.com/linkdata/jaws/jaws"
	"github.com/linkdata/jaws/ui"
)

func TestHandler_ServeHTTP(t *testing.T) {
	pkg.NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	dot := jaws.Tag("123")
	h := ui.NewHandler(rq.TestRequest.Request.Jaws, "testtemplate", dot)
	var buf bytes.Buffer
	var rr httptest.ResponseRecorder
	rr.Body = &buf
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(&rr, r)
	if got := buf.String(); got != `<div id="Jid.1" >123</div>` {
		t.Error(got)
	}
}
