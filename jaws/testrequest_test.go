package jaws

import "net/http"

type testRequest struct {
	*TestRequest
	rw testRequestWriter
}

func newWrappedTestRequest(jw *Jaws, hr *http.Request) *testRequest {
	tr := NewTestRequest(jw, hr)
	if tr == nil {
		return nil
	}
	return &testRequest{
		TestRequest: tr,
		rw: testRequestWriter{
			rq:     tr.Request,
			Writer: tr.ResponseRecorder,
		},
	}
}

func (tr *testRequest) UI(ui UI, params ...any) error    { return tr.rw.UI(ui, params...) }
func (tr *testRequest) Initial() *http.Request           { return tr.rw.Initial() }
func (tr *testRequest) HeadHTML() error                  { return tr.rw.HeadHTML() }
func (tr *testRequest) TailHTML() error                  { return tr.rw.TailHTML() }
func (tr *testRequest) Session() *Session                { return tr.rw.Session() }
func (tr *testRequest) Get(key string) (val any)         { return tr.rw.Get(key) }
func (tr *testRequest) Set(key string, val any)          { tr.rw.Set(key, val) }
func (tr *testRequest) Register(u Updater, p ...any) Jid { return tr.rw.Register(u, p...) }
func (tr *testRequest) Template(name string, dot any, params ...any) error {
	return tr.rw.Template(name, dot, params...)
}
