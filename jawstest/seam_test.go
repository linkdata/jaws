package jawstest

import (
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

func TestNewTestRequest_PanicsAfterClose(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	jw.Close()

	defer func() {
		const want = "jawstest: request could not be claimed (Jaws closed or request retired)"
		if got := recover(); got != want {
			t.Fatalf("panic = %v, want %q", got, want)
		}
	}()
	NewTestRequest(jw, nil)
}
