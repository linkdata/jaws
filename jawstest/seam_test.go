package jawstest

import (
	"net/http"
	"testing"

	"github.com/linkdata/jaws"
)

// TestRepanic exercises both branches of repanic: a nil recovered value (the
// loop exited normally) is ignored, while any other value is re-raised.
func TestRepanic(t *testing.T) {
	repanic(nil) // must not panic

	defer func() {
		if r := recover(); r != "boom" {
			t.Fatalf("repanic did not re-raise the value, got %v", r)
		}
	}()
	repanic("boom")
	t.Fatal("repanic did not panic on a non-nil value")
}

// TestNewTestRequest_NilWhenClaimFails drives the claim-failure path. The
// request cannot be claimed twice, so substituting a constructor that claims it
// up front makes NewTestRequest's own claim attempt fail, returning nil.
func TestNewTestRequest_NilWhenClaimFails(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	orig := newRequest
	t.Cleanup(func() { newRequest = orig })
	newRequest = func(jw *jaws.Jaws, r *http.Request) *jaws.Request {
		rq := orig(jw, r)
		jw.UseRequest(rq.JawsKey, r) // claim it before NewTestRequest can
		return rq
	}

	if tr := NewTestRequest(jw, nil); tr != nil {
		t.Fatal("expected nil when the request cannot be claimed")
	}
}
