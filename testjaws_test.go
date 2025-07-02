package jaws

import (
	"bytes"
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"
)

type testJaws struct {
	*Jaws
	testtmpl *template.Template
	log      bytes.Buffer
}

func newTestJaws() (tj *testJaws) {
	jw, err := New()
	if err != nil {
		panic(err)
	}
	tj = &testJaws{
		Jaws: jw,
	}
	tj.Jaws.Logger = slog.New(slog.NewTextHandler(&tj.log, nil))
	tj.Jaws.MakeAuth = func(r *Request) Auth {
		return defaultAuth{}
	}
	tj.testtmpl = template.Must(template.New("testtemplate").Parse(`{{with $.Dot}}<div id="{{$.Jid}}" {{$.Attrs}}>{{.}}</div>{{end}}`))
	tj.AddTemplateLookuper(tj.testtmpl)

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
	outCh       chan wsMsg
	bcastCh     chan Message
	ctx         context.Context
	cancel      context.CancelFunc
	expectPanic bool
	panicked    bool
	panicVal    any
	*Request
	RequestWriter
}

func (tj *testJaws) newRequest(hr *http.Request) (tr *testRequest) {
	if hr == nil {
		hr = httptest.NewRequest(http.MethodGet, "/", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	hr = hr.WithContext(ctx)
	rr := httptest.NewRecorder()
	rr.Body = &bytes.Buffer{}
	rq := tj.NewRequest(hr)
	if rq == nil || tj.UseRequest(rq.JawsKey, hr) != rq {
		panic("failed to create or use jaws.Request")
	}
	bcastCh := tj.subscribe(rq, 64)
	for i := 0; i <= cap(tj.subCh); i++ {
		tj.subCh <- subscription{} // ensure subscription is processed
	}

	tr = &testRequest{
		hr:            hr,
		rr:            rr,
		jw:            tj,
		readyCh:       make(chan struct{}),
		doneCh:        make(chan struct{}),
		inCh:          make(chan wsMsg),
		outCh:         make(chan wsMsg, cap(bcastCh)),
		bcastCh:       bcastCh,
		ctx:           ctx,
		cancel:        cancel,
		Request:       rq,
		RequestWriter: rq.Writer(rr),
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
		tr.jw.recycle(tr.Request)
	}()

	return
}

func (tr *testRequest) BodyString() string {
	return tr.rr.Body.String()
}

func (tr *testRequest) BodyHTML() template.HTML {
	return template.HTML(tr.BodyString())
}

func (tr *testRequest) Close() {
	tr.cancel()
	tr.jw.Close()
}

func (tr *testRequest) Write(buf []byte) (int, error) {
	return tr.rr.Write(buf)
}

func newTestRequest() (tr *testRequest) {
	tj := newTestJaws()
	return tj.newRequest(nil)
}
