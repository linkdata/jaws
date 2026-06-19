// Package jawstest provides an importable harness for driving a [jaws.Request]'s
// WebSocket message-processing loop in tests.
//
// It lives in its own package, rather than in package jaws, so that
// net/http/httptest stays out of the production build of consumers that import
// github.com/linkdata/jaws. It reaches the request loop through the exported
// [jaws.Jaws.TestServe] hook.
package jawstest

import (
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
//
// OutCh is buffered but must be drained: a test that produces more outbound
// messages than the buffer holds without reading OutCh stalls the loop, and a
// wait on DoneCh after [TestRequest.Close] then never completes.
//
// Recorder is a sink for the test's own rendering, for example as the Writer
// of a ui.RequestWriter; nothing in the harness writes to it.
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
// ([jaws.Jaws.Serve] or [jaws.Jaws.ServeWithTimeout]) to be running, and returns nil if the
// request cannot be created or claimed.
func NewTestRequest(jw *jaws.Jaws, r *http.Request) *TestRequest {
	if r == nil {
		r = httptest.NewRequest(http.MethodGet, "/", nil)
	}
	rr := httptest.NewRecorder()
	rq := newRequest(jw, r)
	// [jaws.Jaws.NewRequest] never returns nil in production (it loops until a key is
	// allocated); the rq == nil guard is defensive against the newRequest seam, while
	// the claim check is the disjunct that fails in practice.
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

// Close stops the test request's processing loop by closing InCh.
//
// It does not wait for the loop to stop; wait on DoneCh for that. Calling
// Close more than once panics.
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
