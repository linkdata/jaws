package jaws

import (
	"net/http"

	"github.com/linkdata/jaws/core/internal/testutil"
	"github.com/linkdata/jaws/core/wire"
)

// TestRequest is a request harness intended for tests.
type TestRequest struct {
	*Request
	*testutil.RequestHarness[Request, wire.WsMsg, wire.WsMsg, wire.Message]
}

// NewTestRequest creates a TestRequest for use when testing.
// Passing nil for hr creates a GET / request with no body.
func NewTestRequest(jw *Jaws, hr *http.Request) (tr *TestRequest) {
	rh := testutil.NewRequestHarness(jw, hr, testutil.RequestHarnessHooks[Jaws, Request, wire.WsMsg, wire.WsMsg, wire.Message]{
		NewRequest: func(jw *Jaws, hr *http.Request) *Request {
			return jw.NewRequest(hr)
		},
		UseRequest: func(jw *Jaws, rq *Request, hr *http.Request) bool {
			return jw.UseRequest(rq.JawsKey, hr) == rq
		},
		Subscribe: func(jw *Jaws, rq *Request, size int) chan wire.Message {
			return jw.subscribe(rq, size)
		},
		PumpSubscriptions: func(jw *Jaws) {
			for i := 0; i <= cap(jw.subCh); i++ {
				jw.subCh <- subscription{}
			}
		},
		Process: func(rq *Request, broadcastMsgCh chan wire.Message, incomingMsgCh <-chan wire.WsMsg, outboundMsgCh chan<- wire.WsMsg) {
			rq.process(broadcastMsgCh, incomingMsgCh, outboundMsgCh)
		},
		Recycle: func(jw *Jaws, rq *Request) {
			jw.recycle(rq)
		},
	})
	if rh != nil {
		tr = &TestRequest{
			Request:        rh.Req,
			RequestHarness: rh,
		}
	}
	return
}
