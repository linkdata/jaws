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

func (e ErrNoWebSocketRequest) Is(target error) (yes bool) {
	_, yes = target.(ErrNoWebSocketRequest)
	return
}

func newErrNoWebSocketRequest(rq *Request) error {
	return ErrNoWebSocketRequest{Addr: rq.remoteIP}
}
