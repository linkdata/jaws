package jaws

import (
	"bytes"
	"context"
	"html/template"
	"log"
	"net/http"
	"net/http/httptest"
	"time"
)

type testJaws struct {
	*Jaws
	log bytes.Buffer
}

func newTestJaws() (tj *testJaws) {
	tj = &testJaws{
		Jaws: New(),
	}
	tj.Jaws.Logger = log.New(&tj.log, "", 0)
	tj.Template = template.Must(template.New("testtemplate").Parse(`{{with $.Dot}}<div id="{{$.Jid}}"{{$.Attrs}}>{{.}}</div>{{end}}`))
	tj.Jaws.updateTicker = time.NewTicker(time.Millisecond)
	go tj.Serve()
	return
}

type testRequest struct {
	hr          *http.Request
	rr          *httptest.ResponseRecorder
	jw          *testJaws
	readyCh     chan struct{}
	doneCh      chan struct{}
	inCh        chan wsMsg
	outCh       chan string
	bcastCh     chan Message
	ctx         context.Context
	cancel      context.CancelFunc
	expectPanic bool
	panicked    bool
	panicVal    any
	*Request
}

func (tj *testJaws) newRequest(hr *http.Request) (tr *testRequest) {
	if hr == nil {
		hr = httptest.NewRequest(http.MethodGet, "/", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	hr = hr.WithContext(ctx)
	rr := httptest.NewRecorder()
	rr.Body = &bytes.Buffer{}
	rq := tj.NewRequest(rr, hr)
	if rq == nil || tj.UseRequest(rq.JawsKey, hr) != rq {
		panic("failed to create or use jaws.Request")
	}
	bcastCh := tj.subscribe(rq, 64)
	for i := 0; i <= cap(tj.subCh); i++ {
		tj.subCh <- subscription{} // ensure subscription is processed
	}

	tr = &testRequest{
		hr:      hr,
		rr:      rr,
		jw:      tj,
		readyCh: make(chan struct{}),
		doneCh:  make(chan struct{}),
		inCh:    make(chan wsMsg),
		outCh:   make(chan string, cap(bcastCh)),
		bcastCh: bcastCh,
		ctx:     ctx,
		cancel:  cancel,
		Request: rq,
	}

	go func() {
		defer func() {
			if tr.expectPanic {
				if tr.panicVal = recover(); tr.panicVal != nil {
					tr.panicked = true
				}
			}
			close(tr.doneCh)
		}()
		close(tr.readyCh)
		tr.process(tr.bcastCh, tr.inCh, tr.outCh) // usubs from bcase, closes outCh
		tr.recycle()
	}()

	return
}

func (tr *testRequest) BodyString() string {
	return tr.rr.Body.String()
}

func (tr *testRequest) BodyHtml() template.HTML {
	return template.HTML(tr.BodyString())
}

func (tr *testRequest) Close() {
	tr.cancel()
	tr.jw.Close()
}

func newTestRequest() (tr *testRequest) {
	tj := newTestJaws()
	return tj.newRequest(nil)
}
