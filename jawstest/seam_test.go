package jawstest

import (
	"net/http"
	"strings"
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

// TestNewTestRequest_PanicsWhenClaimFails drives the claim-failure path. The
// request cannot be claimed twice, so substituting a constructor that claims it
// up front makes NewTestRequest's own claim attempt fail, which panics.
func TestNewTestRequest_PanicsWhenClaimFails(t *testing.T) {
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

	defer func() {
		switch r := recover().(type) {
		case nil:
			t.Fatal("expected a panic when the request cannot be claimed")
		case string:
			if !strings.Contains(r, "could not be claimed") {
				t.Fatalf("recovered %q, want a 'could not be claimed' panic", r)
			}
		default:
			t.Fatalf("recovered %v (%T), want a string panic", r, r)
		}
	}()
	NewTestRequest(jw, nil)
	t.Fatal("NewTestRequest returned instead of panicking")
}
