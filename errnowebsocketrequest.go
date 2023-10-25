package jaws

import (
	"fmt"
	"net/netip"
)

type ErrNoWebSocketRequest struct {
	netip.Addr
}

func (e ErrNoWebSocketRequest) Error() string {
	return fmt.Sprintf("no WebSocket request received from %v", e.Addr)
}

func newErrNoWebSocketRequest(rq *Request) error {
	return ErrNoWebSocketRequest{Addr: rq.remoteIP}
}
