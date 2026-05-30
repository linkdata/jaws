// Package jawstest provides an importable harness for driving a [jaws.Request]'s
// WebSocket message-processing loop in tests.
//
// It lives in its own package, rather than in package jaws, so that
// net/http/httptest stays out of the production build of consumers that import
// github.com/linkdata/jaws. It reaches the request loop through the exported
// [jaws.Jaws.TestServe] hook.
package jawstest

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/wire"
)

// TestRequest is a request harness intended for tests.
//
// The embedded [jaws.Request] provides the usual request methods (NewElement,
// JawsKeyString, and so on). The channels expose the loop's wiring: send incoming
// WebSocket messages on InCh, read outbound messages from OutCh, and inject
// broadcasts on BcastCh. ReadyCh is closed once the loop is running and DoneCh
// once it has stopped.
type TestRequest struct {
	*jaws.Request
	Recorder *httptest.ResponseRecorder
	ReadyCh  chan struct{}
	DoneCh   chan struct{}
	InCh     chan wire.WsMsg
	OutCh    chan wire.WsMsg
	BcastCh  chan wire.Message
}

// newRequest constructs the pending [jaws.Request] that NewTestRequest then
// claims and serves. It is a package variable so tests can substitute a
// constructor returning an already-claimed request, exercising the
// claim-failure path in NewTestRequest.
var newRequest = (*jaws.Jaws).NewRequest

// repanic re-raises a panic value recovered from the request's processing loop.
// A nil value means the loop exited normally and is ignored; any other value is
// re-raised so an unexpected loop panic surfaces instead of being swallowed.
func repanic(recovered any) {
	if recovered != nil {
		panic(recovered)
	}
}

// NewTestRequest creates a TestRequest for use when testing. Passing nil for r
// creates a GET / request with no body. It requires the Jaws processing loop
// ([jaws.Jaws.Serve] or ServeWithTimeout) to be running, and returns nil if the
// request cannot be created or claimed.
func NewTestRequest(jw *jaws.Jaws, r *http.Request) *TestRequest {
	if r == nil {
		r = httptest.NewRequest(http.MethodGet, "/", nil)
	}
	rr := httptest.NewRecorder()
	rr.Body = &bytes.Buffer{}
	rq := newRequest(jw, r)
	if rq == nil || jw.UseRequest(rq.JawsKey, r) != rq {
		return nil
	}
	tr := &TestRequest{
		Request:  rq,
		Recorder: rr,
	}
	// This harness does not expect loop panics, so re-raise any so they surface
	// instead of being silently swallowed.
	tr.InCh, tr.OutCh, tr.BcastCh, tr.ReadyCh, tr.DoneCh = jw.TestServe(rq, repanic)
	return tr
}

// Close stops the test request's processing loop.
func (tr *TestRequest) Close() {
	close(tr.InCh)
}

// BodyString returns the recorded response body with surrounding whitespace removed.
func (tr *TestRequest) BodyString() string {
	return strings.TrimSpace(tr.Recorder.Body.String())
}

// BodyHTML returns the recorded response body as trusted HTML.
func (tr *TestRequest) BodyHTML() template.HTML {
	return template.HTML(tr.BodyString()) /* #nosec G203 */
}
