package jaws

import (
	"fmt"
	"net/netip"
)

// ErrWebSocketIPMismatch is returned when the WebSocket callback for a
// [Request] arrives from a different client IP than the initial HTTP request.
var ErrWebSocketIPMismatch errWebSocketIPMismatch

type errWebSocketIPMismatch struct {
	JawsKey  string
	Expected netip.Addr
	Actual   netip.Addr
}

func (e errWebSocketIPMismatch) Error() string {
	return fmt.Sprintf("/jaws/%s: expected IP %q, got %q", e.JawsKey, e.Expected.String(), e.Actual.String())
}

func (e errWebSocketIPMismatch) Is(target error) bool {
	return target == ErrWebSocketIPMismatch
}

// newErrWebSocketIPMismatchLocked reads rq fields; caller must hold rq.mu.
//
// It reads rq.JawsKey directly rather than via [Request.JawsKeyString], which
// takes rq.mu and would deadlock here since the caller already holds it.
func newErrWebSocketIPMismatchLocked(rq *Request, actual netip.Addr) error {
	return errWebSocketIPMismatch{JawsKey: rq.JawsKey.String(), Expected: rq.remoteIP, Actual: actual}
}
