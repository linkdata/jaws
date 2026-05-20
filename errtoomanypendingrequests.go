package jaws

import (
	"fmt"
	"net/netip"
)

// ErrTooManyPendingRequests indicates an older pending Request was evicted
// because its client IP had reached [Jaws.MaxPendingRequestsPerIP].
var ErrTooManyPendingRequests errTooManyPendingRequests

type errTooManyPendingRequests struct {
	Addr  netip.Addr
	Limit int
}

func (e errTooManyPendingRequests) Error() string {
	return fmt.Sprintf("too many pending requests from %v (limit %d)", e.Addr, e.Limit)
}

func (e errTooManyPendingRequests) Is(target error) bool {
	return target == ErrTooManyPendingRequests
}

func newErrTooManyPendingRequests(remoteIP netip.Addr, limit int) error {
	return errTooManyPendingRequests{Addr: remoteIP, Limit: limit}
}
