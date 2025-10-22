package jaws

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testJaws struct {
	*Jaws
	testtmpl *template.Template
	log      bytes.Buffer
}

// type testRequest = TestRequest

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

func (tj *testJaws) newRequest(hr *http.Request) (tr *TestRequest) {
	return NewTestRequest(tj.Jaws, hr)
}

/*type testRequest struct {
	rr          *httptest.ResponseRecorder
	ReadyCh     chan struct{}
	DoneCh      chan struct{}
	InCh        chan wsMsg
	OutCh       chan wsMsg
	BcastCh     chan Message
	ctx         context.Context
	cancel      context.CancelFunc
	ExpectPanic bool
	Panicked    bool
	PanicVal    any
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
		rr:            rr,
		ReadyCh:       make(chan struct{}),
		DoneCh:        make(chan struct{}),
		InCh:          make(chan wsMsg),
		OutCh:         make(chan wsMsg, cap(bcastCh)),
		BcastCh:       bcastCh,
		ctx:           ctx,
		cancel:        cancel,
		Request:       rq,
		RequestWriter: rq.Writer(rr),
	}

	go func() {
		defer func() {
			if tr.ExpectPanic {
				if tr.PanicVal = recover(); tr.PanicVal != nil {
					tr.Panicked = true
				}
			} else {
				close(tr.InCh)
			}
			close(tr.DoneCh)
		}()
		close(tr.ReadyCh)
		tr.process(tr.BcastCh, tr.InCh, tr.OutCh) // usubs from bcase, closes outCh
		tr.Jaws.recycle(tr.Request)
	}()

	return
}

func (tr *testRequest) BodyString() string {
	return tr.rr.Body.String()
}

func (tr *testRequest) BodyHTML() template.HTML {
	return template.HTML(strings.TrimSpace(tr.BodyString()))
}

func (tr *testRequest) Close() {
	tr.cancel()
	tr.Jaws.Close()
}

func (tr *testRequest) Write(buf []byte) (int, error) {
	return tr.rr.Write(buf)
}

func (tr *testRequest) GetElementByJid(jid Jid) (e *Element) {
	tr.Request.mu.RLock()
	defer tr.Request.mu.RUnlock()
	e = tr.Request.getElementByJidLocked(jid)
	return
}

func newTestRequest(t) (tr *testRequest) {
	tj := newTestJaws()
	return tj.newRequest(httptest.NewRequest("GET", "/", nil))
}*/

func newTestRequest(t *testing.T) (tr *TestRequest) {
	tj := newTestJaws()
	if t != nil {
		t.Helper()
		t.Cleanup(tj.Close)
	}
	return NewTestRequest(tj.Jaws, httptest.NewRequest("GET", "/", nil))
}
