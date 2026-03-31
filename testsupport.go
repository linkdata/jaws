package jaws

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/linkdata/jaws/jawswire"
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
	InCh        chan jawswire.WsMsg
	OutCh       chan jawswire.WsMsg
	BcastCh     chan jawswire.Message
	ExpectPanic bool
	Panicked    bool
	PanicVal    any
}

func newRequestHarness(jw *Jaws, hr *http.Request) (rh *requestHarness) {
	if hr == nil {
		hr = httptest.NewRequest(http.MethodGet, "/", nil)
	}
	rr := httptest.NewRecorder()
	rr.Body = &bytes.Buffer{}
	rq := jw.NewRequest(hr)
	if rq == nil || jw.UseRequest(rq.JawsKey, hr) != rq {
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
		InCh:             make(chan jawswire.WsMsg),
		OutCh:            make(chan jawswire.WsMsg, cap(bcastCh)),
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

func (rh *requestHarness) Close() {
	close(rh.InCh)
}

func (rh *requestHarness) BodyString() string {
	return strings.TrimSpace(rh.Body.String())
}

func (rh *requestHarness) BodyHTML() template.HTML {
	return template.HTML(rh.BodyString()) /* #nosec G203 */
}

// NewTestRequest creates a TestRequest for use when testing.
// Passing nil for hr creates a GET / request with no body.
func NewTestRequest(jw *Jaws, hr *http.Request) (tr *TestRequest) {
	rh := newRequestHarness(jw, hr)
	if rh != nil {
		tr = &TestRequest{
			Request:        rh.Req,
			requestHarness: rh,
		}
	}
	return
}
