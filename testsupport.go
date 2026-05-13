package jaws

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/linkdata/jaws/lib/wire"
)

// TestRequest is a request harness intended for tests.
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
	bcastCh := jw.subscribe(rq, 64)
	for i := 0; i <= cap(jw.subCh); i++ {
		jw.subCh <- subscription{}
	}
	rh = &requestHarness{
		Req:              rq,
		ResponseRecorder: rr,
		ReadyCh:          make(chan struct{}),
		DoneCh:           make(chan struct{}),
		InCh:             make(chan wire.WsMsg),
		OutCh:            make(chan wire.WsMsg, cap(bcastCh)),
		BcastCh:          bcastCh,
	}
	go func() {
		defer func() {
			if rh.ExpectPanic {
				if rh.PanicVal = recover(); rh.PanicVal != nil {
					rh.Panicked = true
				}
			}
			close(rh.DoneCh)
		}()
		close(rh.ReadyCh)
		rq.process(rh.BcastCh, rh.InCh, rh.OutCh)
		jw.recycle(rq)
	}()
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
// Passing nil for hr creates a GET / request with no body.
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
