package jawstest

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/linkdata/jaws/jaws"
	"github.com/linkdata/jaws/ui"
)

type TestRequest struct {
	*jaws.Request
	*httptest.ResponseRecorder
	ui.RequestWriter
	ReadyCh     chan struct{}
	DoneCh      chan struct{}
	InCh        chan jaws.WsMsg
	OutCh       chan jaws.WsMsg
	BcastCh     chan jaws.Message
	ExpectPanic bool
	Panicked    bool
	PanicVal    any
}

// NewTestRequest creates a TestRequest for use when testing.
// Passing nil for hr will create a "GET /" request with no body.
//
// If NewRequest() or UseRequest() fails, it returns nil.
func NewTestRequest(jw *jaws.Jaws, hr *http.Request) (tr *TestRequest) {
	if hr == nil {
		hr = httptest.NewRequest(http.MethodGet, "/", nil)
	}
	rr := httptest.NewRecorder()
	rr.Body = &bytes.Buffer{}
	rq := jw.NewRequest(hr)
	if rq != nil && jw.UseRequest(rq.JawsKey, hr) == rq {
		bcastCh := jw.subscribe(rq, 64)
		for i := 0; i <= cap(jw.subCh); i++ {
			jw.subCh <- jaws.subscription{} // ensure subscription is processed
		}

		tr = &TestRequest{
			ReadyCh:          make(chan struct{}),
			DoneCh:           make(chan struct{}),
			InCh:             make(chan jaws.WsMsg),
			OutCh:            make(chan jaws.WsMsg, cap(bcastCh)),
			BcastCh:          bcastCh,
			Request:          rq,
			RequestWriter:    ui.RequestWriter{Request: rq, Writer: rr},
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
			tr.process(tr.BcastCh, tr.InCh, tr.OutCh) // unsubs from bcast, closes outCh
			jw.recycle(tr.Request)
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
