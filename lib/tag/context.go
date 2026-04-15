package tag

import (
	"context"
	"net/http"
)

type Context interface {
	// Initial returns the Request's initial HTTP request, or nil.
	Initial() (r *http.Request)
	// Get returns the jaws session value for the key, or nil.
	Get(key string) any
	// Set sets the jaws session value for the key.
	Set(key string, val any)
	// Context returns the Request's context.
	Context() (ctx context.Context)
	// Log sends an error to the Logger set in the Jaws.
	// Has no effect if the err is nil or the Logger is nil.
	// Returns err.
	Log(err error) error
	// MustLog sends an error to the Logger set in the Jaws or
	// panics with the given error if no Logger is set.
	// Has no effect if the err is nil.
	MustLog(err error)
}
