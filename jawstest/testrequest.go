package jawstest

import (
	"net/http"

	"github.com/linkdata/jaws"
)

// TestRequest aliases jaws.TestRequest for cross-package integration tests.
//
// Exposed through jawstest for convenience.
type TestRequest = jaws.TestRequest

// NewTestRequest forwards to jaws.NewTestRequest.
func NewTestRequest(jw *jaws.Jaws, hr *http.Request) *TestRequest {
	return jaws.NewTestRequest(jw, hr)
}
