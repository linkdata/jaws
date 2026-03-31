package testkit

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/wire"
)

// TestRequest is a request harness intended for tests.
//
// Exposed for testing only.
type TestRequest struct {
	*core.Request
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

// NewTestRequest creates a TestRequest for use when testing.
// Passing nil for hr will create a "GET /" request with no body.
//
// Exposed for testing only.
func NewTestRequest(jw *core.Jaws, hr *http.Request) (tr *TestRequest) {
	if hr == nil {
		hr = httptest.NewRequest(http.MethodGet, "/", nil)
	}
	rr := httptest.NewRecorder()
	rr.Body = &bytes.Buffer{}
	rq := jw.NewRequest(hr)
	if rq != nil && jw.UseRequest(rq.JawsKey, hr) == rq {
		bcastCh := jw.SubscribeForTest(rq, 64)
		jw.PumpSubscriptionsForTest() // ensure subscription is processed

		tr = &TestRequest{
			ReadyCh:          make(chan struct{}),
			DoneCh:           make(chan struct{}),
			InCh:             make(chan wire.WsMsg),
			OutCh:            make(chan wire.WsMsg, cap(bcastCh)),
			BcastCh:          bcastCh,
			Request:          rq,
			ResponseRecorder: rr,
		}

		go func() {
			defer func() {
				if tr.ExpectPanic {
					if tr.PanicVal = recover(); tr.PanicVal != nil {
						tr.Panicked = true
					}
				}
				close(tr.DoneCh)
			}()
			close(tr.ReadyCh)
			tr.ProcessForTest(tr.BcastCh, tr.InCh, tr.OutCh) // unsubs from bcast, closes outCh
			jw.RecycleForTest(tr.Request)
		}()
	}

	return
}

func (tr *TestRequest) Close() {
	close(tr.InCh)
}

func (tr *TestRequest) BodyString() string {
	return strings.TrimSpace(tr.Body.String())
}

func (tr *TestRequest) BodyHTML() template.HTML {
	return template.HTML(tr.BodyString()) /* #nosec G203 */
}
