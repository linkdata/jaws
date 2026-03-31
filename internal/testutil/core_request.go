package testutil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "github.com/linkdata/jaws/core"
)

// NewCoreRequest creates a core.Jaws instance and a GET / request bound to it.
func NewCoreRequest(t *testing.T) (*core.Jaws, *core.Request) {
	t.Helper()

	jw, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		t.Fatal("nil request")
	}
	return jw, rq
}

// NewCoreSessionBoundRequest creates a core.Request with an attached session.
func NewCoreSessionBoundRequest(t *testing.T) (*core.Jaws, *core.Request) {
	t.Helper()

	jw, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	if sess := jw.NewSession(rr, hr); sess == nil {
		t.Fatal("expected session")
	}

	rq := jw.NewRequest(hr)
	if rq == nil {
		t.Fatal("expected request")
	}

	return jw, rq
}
