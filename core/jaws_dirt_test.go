package core

import (
	"reflect"
	"testing"
)

func TestJaws_distributeDirt_AscendingOrder(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	rq := &Request{}
	jw.mu.Lock()
	jw.requests[1] = rq
	jw.dirty[Tag("fourth")] = 4
	jw.dirty[Tag("second")] = 2
	jw.dirty[Tag("fifth")] = 5
	jw.dirty[Tag("first")] = 1
	jw.dirty[Tag("third")] = 3
	jw.dirtOrder = 5
	jw.mu.Unlock()

	if got, want := jw.distributeDirt(), 5; got != want {
		t.Fatalf("distributeDirt() = %d, want %d", got, want)
	}

	rq.mu.RLock()
	got := append([]any(nil), rq.todoDirt...)
	rq.mu.RUnlock()

	want := []any{
		Tag("first"),
		Tag("second"),
		Tag("third"),
		Tag("fourth"),
		Tag("fifth"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dirty tags = %#v, want %#v", got, want)
	}
}
