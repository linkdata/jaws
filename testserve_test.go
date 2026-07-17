package jaws

import (
	"strings"
	"testing"
)

func TestTestServe_PanicsWhenJawsAlreadyClosed(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	rq := jw.NewRequest(nil)
	jw.Close()

	defer func() {
		got := recover()
		message, ok := got.(string)
		if !ok || !strings.Contains(message, "Jaws instance is closed") {
			t.Fatalf("panic = %v, want closed-Jaws diagnostic", got)
		}
	}()
	jw.TestServe(rq, func(any) {})
	t.Fatal("TestServe did not panic for a closed Jaws instance")
}
