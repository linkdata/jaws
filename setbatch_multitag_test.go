package jaws

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestSetBatchMultiTagAndOverlappingDestinations(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()
	tr := NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("failed to create test request")
	}
	defer func() {
		tr.Close()
		<-tr.DoneCh
	}()
	<-tr.ReadyCh

	a, b := tag.Tag("A"), tag.Tag("B")
	onlyA := tr.NewElement(&testUi{})
	onlyB := tr.NewElement(&testUi{})
	both := tr.NewElement(&testUi{})
	onlyA.Tag(a)
	onlyB.Tag(b)
	both.Tag(a, b)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	readPhase := func(marker string) map[Jid][]string {
		t.Helper()
		got := make(map[Jid][]string)
		for {
			select {
			case msg, ok := <-tr.OutCh:
				if !ok {
					t.Fatalf("request ended before %q barrier: %v", marker, context.Cause(tr.Context()))
				}
				if msg.What == what.Alert {
					if msg.Data != marker {
						t.Fatalf("barrier = %#v, want %q", msg, marker)
					}
					return got
				}
				if msg.What != what.Set {
					t.Fatalf("message before %q barrier = %#v, want Set", marker, msg)
				}
				got[msg.Jid] = append(got[msg.Jid], msg.Data)
			case <-ctx.Done():
				t.Fatalf("timed out waiting for %q barrier: %v", marker, context.Cause(tr.Context()))
			}
		}
	}

	// Equal tag sets can arrive in different orders. Each selected Element gets
	// both paths once, in mutation order, including the Element tagged twice.
	jw.Broadcast(wire.Message{Dest: []any{a, b}, What: what.Set, Data: "x=1"})
	jw.Broadcast(wire.Message{Dest: []any{b, a}, What: what.Set, Data: "y=2"})
	jw.Broadcast(wire.Message{What: what.Alert, Data: "multi-tag"})
	want := map[Jid][]string{
		onlyA.Jid(): {"x=1", "y=2"},
		onlyB.Jid(): {"x=1", "y=2"},
		both.Jid():  {"x=1", "y=2"},
	}
	if got := readPhase("multi-tag"); !reflect.DeepEqual(got, want) {
		t.Fatalf("multi-tag Sets = %#v, want %#v", got, want)
	}

	// A destination-wide regrouping would reorder at least one of these writes
	// for the Element that matches both A and B.
	for _, msg := range []wire.Message{
		{Dest: a, What: what.Set, Data: "p=1"},
		{Dest: b, What: what.Set, Data: "q=2"},
		{Dest: a, What: what.Set, Data: "q=3"},
		{Dest: b, What: what.Set, Data: "p=4"},
	} {
		jw.Broadcast(msg)
	}
	jw.Broadcast(wire.Message{What: what.Alert, Data: "overlap"})
	want = map[Jid][]string{
		onlyA.Jid(): {"p=1", "q=3"},
		onlyB.Jid(): {"q=2", "p=4"},
		both.Jid():  {"p=1", "q=2", "q=3", "p=4"},
	}
	if got := readPhase("overlap"); !reflect.DeepEqual(got, want) {
		t.Fatalf("overlapping tag Sets = %#v, want %#v", got, want)
	}
	if cause := context.Cause(tr.Context()); cause != nil {
		t.Fatalf("request cancelled: %v", cause)
	}
}
