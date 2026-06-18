package jaws

import "errors"

// ErrRequestOverloaded indicates a [Request] was torn down because it could not keep
// up with the messages addressed to it.
//
// A Request is overloaded when its buffered broadcast channel or its internal
// event-call channel fills before it can drain them. Rather than silently dropping
// messages, which could leave the browser and backend in inconsistent and
// nonreproducible states, the Request is cancelled. The cancellation cause reachable
// via [context.Cause] on [Request.Context] wraps this sentinel, so it can be matched
// with [errors.Is]; the wrapped text identifies which channel overflowed.
var ErrRequestOverloaded = errors.New("request overloaded")
