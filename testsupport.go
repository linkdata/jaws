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

// SubscribeForTest subscribes rq to broadcasts with the given channel size.
//
// It is intended for use by test helpers.
func (jw *Jaws) SubscribeForTest(rq *Request, size int) chan wire.Message {
	return jw.subscribe(rq, size)
}

// PumpSubscriptionsForTest pushes empty subscriptions through the internal queue.
//
// It is intended for use by test helpers.
func (jw *Jaws) PumpSubscriptionsForTest() {
	for i := 0; i <= cap(jw.subCh); i++ {
		jw.subCh <- subscription{}
	}
}

// ProcessForTest runs the request processing loop with explicit channels.
//
// It is intended for use by test helpers.
func (rq *Request) ProcessForTest(broadcastMsgCh chan wire.Message, incomingMsgCh <-chan wire.WsMsg, outboundMsgCh chan<- wire.WsMsg) {
	rq.process(broadcastMsgCh, incomingMsgCh, outboundMsgCh)
}

// RecycleForTest recycles rq back into the request pool.
//
// It is intended for use by test helpers.
func (jw *Jaws) RecycleForTest(rq *Request) {
	jw.recycle(rq)
}
