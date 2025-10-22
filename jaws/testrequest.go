package jaws

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
)

type TestRequest struct {
	*Request
	*httptest.ResponseRecorder
	RequestWriter
	ReadyCh     chan struct{}
	DoneCh      chan struct{}
	InCh        chan wsMsg
	OutCh       chan wsMsg
	BcastCh     chan Message
	ExpectPanic bool
	Panicked    bool
	PanicVal    any
}

func NewTestRequest(jw *Jaws, hr *http.Request) (tr *TestRequest) {
	rr := httptest.NewRecorder()
	rr.Body = &bytes.Buffer{}
	rq := jw.NewRequest(hr)
	if rq == nil || jw.UseRequest(rq.JawsKey, hr) != rq {
		panic("failed to create or use jaws.Request")
	}
	bcastCh := jw.subscribe(rq, 64)
	for i := 0; i <= cap(jw.subCh); i++ {
		jw.subCh <- subscription{} // ensure subscription is processed
	}

	tr = &TestRequest{
		ReadyCh:          make(chan struct{}),
		DoneCh:           make(chan struct{}),
		InCh:             make(chan wsMsg),
		OutCh:            make(chan wsMsg, cap(bcastCh)),
		BcastCh:          bcastCh,
		Request:          rq,
		RequestWriter:    rq.Writer(rr),
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

	return
}

func (tr *TestRequest) Close() {
	close(tr.InCh)
}

func (tr *TestRequest) BodyString() string {
	return tr.Body.String()
}

func (tr *TestRequest) BodyHTML() template.HTML {
	return template.HTML(strings.TrimSpace(tr.BodyString()))
}
