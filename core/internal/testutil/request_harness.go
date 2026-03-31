package testutil

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
)

// RequestHarness is a request harness intended for tests.
type RequestHarness[R any, In any, Out any, B any] struct {
	Req *R
	*httptest.ResponseRecorder
	ReadyCh     chan struct{}
	DoneCh      chan struct{}
	InCh        chan In
	OutCh       chan Out
	BcastCh     chan B
	ExpectPanic bool
	Panicked    bool
	PanicVal    any
}

// RequestHarnessHooks contains callbacks used by NewRequestHarness.
type RequestHarnessHooks[J any, R any, In any, Out any, B any] struct {
	NewRequest         func(*J, *http.Request) *R
	UseRequest         func(*J, *R, *http.Request) bool
	Subscribe          func(*J, *R, int) chan B
	PumpSubscriptions  func(*J)
	Process            func(*R, chan B, <-chan In, chan<- Out)
	Recycle            func(*J, *R)
	DefaultRequestPath string
}

// NewRequestHarness creates a RequestHarness for use when testing.
// Passing nil for hr creates a GET / request with no body.
func NewRequestHarness[J any, R any, In any, Out any, B any](jw *J, hr *http.Request, hooks RequestHarnessHooks[J, R, In, Out, B]) (tr *RequestHarness[R, In, Out, B]) {
	if hooks.NewRequest == nil || hooks.UseRequest == nil || hooks.Subscribe == nil || hooks.PumpSubscriptions == nil || hooks.Process == nil || hooks.Recycle == nil {
		panic("missing RequestHarnessHooks callback")
	}
	if hooks.DefaultRequestPath == "" {
		hooks.DefaultRequestPath = "/"
	}
	if hr == nil {
		hr = httptest.NewRequest(http.MethodGet, hooks.DefaultRequestPath, nil)
	}
	rr := httptest.NewRecorder()
	rr.Body = &bytes.Buffer{}
	rq := hooks.NewRequest(jw, hr)
	if rq == nil || !hooks.UseRequest(jw, rq, hr) {
		return nil
	}

	bcastCh := hooks.Subscribe(jw, rq, 64)
	hooks.PumpSubscriptions(jw) // ensure subscription is processed

	tr = &RequestHarness[R, In, Out, B]{
		ReadyCh:          make(chan struct{}),
		DoneCh:           make(chan struct{}),
		InCh:             make(chan In),
		OutCh:            make(chan Out, cap(bcastCh)),
		BcastCh:          bcastCh,
		Req:              rq,
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
		hooks.Process(tr.Req, tr.BcastCh, tr.InCh, tr.OutCh) // unsubs from bcast, closes outCh
		hooks.Recycle(jw, tr.Req)
	}()

	return tr
}

// Close closes the harness input channel.
func (tr *RequestHarness[R, In, Out, B]) Close() {
	close(tr.InCh)
}

// BodyString returns the trimmed body content.
func (tr *RequestHarness[R, In, Out, B]) BodyString() string {
	return strings.TrimSpace(tr.Body.String())
}

// BodyHTML returns the trimmed body content as trusted HTML.
func (tr *RequestHarness[R, In, Out, B]) BodyHTML() template.HTML {
	return template.HTML(tr.BodyString()) /* #nosec G203 */
}
