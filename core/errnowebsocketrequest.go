package core

import (
	"fmt"
	"net/netip"
)

// ErrNoWebSocketRequest is returned when the WebSocket callback was not received
// within the timeout period. Most common reason is that client is not using
// Javascript.
var ErrNoWebSocketRequest errNoWebSocketRequest

type errNoWebSocketRequest struct {
	netip.Addr
}

func (e errNoWebSocketRequest) Error() string {
	return fmt.Sprintf("no WebSocket request received from %v", e.Addr)
}

func (e errNoWebSocketRequest) Is(target error) bool {
	return target == ErrNoWebSocketRequest
}

func newErrNoWebSocketRequest(rq *Request) error {
	return errNoWebSocketRequest{Addr: rq.remoteIP}
}
