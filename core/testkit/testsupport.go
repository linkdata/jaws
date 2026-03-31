package testkit

import (
	"net/http"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/internal/testutil"
	"github.com/linkdata/jaws/core/wire"
)

// TestRequest is a request harness intended for tests.
//
// Exposed for testing only.
type TestRequest struct {
	*core.Request
	*testutil.RequestHarness[core.Request, wire.WsMsg, wire.WsMsg, wire.Message]
}

// NewTestRequest creates a TestRequest for use when testing.
// Passing nil for hr will create a "GET /" request with no body.
//
// Exposed for testing only.
func NewTestRequest(jw *core.Jaws, hr *http.Request) (tr *TestRequest) {
	rh := testutil.NewRequestHarness(jw, hr, testutil.RequestHarnessHooks[core.Jaws, core.Request, wire.WsMsg, wire.WsMsg, wire.Message]{
		NewRequest: func(jw *core.Jaws, hr *http.Request) *core.Request {
			return jw.NewRequest(hr)
		},
		UseRequest: func(jw *core.Jaws, rq *core.Request, hr *http.Request) bool {
			return jw.UseRequest(rq.JawsKey, hr) == rq
		},
		Subscribe: func(jw *core.Jaws, rq *core.Request, size int) chan wire.Message {
			return jw.SubscribeForTest(rq, size)
		},
		PumpSubscriptions: func(jw *core.Jaws) {
			jw.PumpSubscriptionsForTest()
		},
		Process: func(rq *core.Request, broadcastMsgCh chan wire.Message, incomingMsgCh <-chan wire.WsMsg, outboundMsgCh chan<- wire.WsMsg) {
			rq.ProcessForTest(broadcastMsgCh, incomingMsgCh, outboundMsgCh)
		},
		Recycle: func(jw *core.Jaws, rq *core.Request) {
			jw.RecycleForTest(rq)
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
