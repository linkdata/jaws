package jaws

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/linkdata/jaws/lib/wire"
)

// TestRequest is a request harness intended for the jaws package's own tests.
// The importable harness for other packages lives in
// github.com/linkdata/jaws/jawstest.
type TestRequest struct {
	*Request
	*requestHarness
}

type requestHarness struct {
	Req *Request
	*httptest.ResponseRecorder
	ReadyCh     chan struct{}
	DoneCh      chan struct{}
	InCh        chan wire.WsMsg
	OutCh       chan wire.WsMsg
	BcastCh     chan wire.Message
	ExpectPanic bool
	Panicked    bool
	PanicVal    any
}

func newRequestHarness(jw *Jaws, r *http.Request) (rh *requestHarness) {
	if r == nil {
		r = httptest.NewRequest(http.MethodGet, "/", nil)
	}
	rr := httptest.NewRecorder()
	rr.Body = &bytes.Buffer{}
	rq := jw.NewRequest(r)
	if rq == nil || jw.UseRequest(rq.JawsKey, r) != rq {
		return nil
	}
	rh = &requestHarness{
		Req:              rq,
		ResponseRecorder: rr,
	}
	// The subscribe/process/recycle dance lives in jw.TestServe. The onPanic
	// callback adds this harness's panic-expectation support: an expected panic is
	// captured for inspection while an unexpected one is re-raised exactly as
	// before. It reads ExpectPanic lazily so tests may set it after construction.
	rh.InCh, rh.OutCh, rh.BcastCh, rh.ReadyCh, rh.DoneCh = jw.TestServe(rq, func(recovered any) {
		if recovered == nil {
			return
		}
		if rh.ExpectPanic {
			rh.PanicVal = recovered
			rh.Panicked = true
			return
		}
		panic(recovered)
	})
	return
}

// Close stops the test request's processing loop.
func (rh *requestHarness) Close() {
	close(rh.InCh)
}

// BodyString returns the recorded response body with surrounding whitespace removed.
func (rh *requestHarness) BodyString() string {
	return strings.TrimSpace(rh.Body.String())
}

// BodyHTML returns the recorded response body as trusted HTML.
func (rh *requestHarness) BodyHTML() template.HTML {
	return template.HTML(rh.BodyString()) /* #nosec G203 */
}

// NewTestRequest creates a TestRequest for use when testing.
// Passing nil for r creates a GET / request with no body.
func NewTestRequest(jw *Jaws, r *http.Request) (tr *TestRequest) {
	rh := newRequestHarness(jw, r)
	if rh != nil {
		tr = &TestRequest{
			Request:        rh.Req,
			requestHarness: rh,
		}
	}
	return
}
