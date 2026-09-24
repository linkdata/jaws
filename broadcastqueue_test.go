package jaws

import (
	"testing"

	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestBroadcastQueueFullSetPolicy(t *testing.T) {
	firstDest := &struct{ value int }{1}
	secondDest := &struct{ value int }{2}
	set := func(dest any, data string) wire.Message {
		return wire.Message{Dest: dest, What: what.Set, Data: data}
	}
	alert := wire.Message{Dest: firstDest, What: what.Alert, Data: "alert"}
	for _, tc := range []struct {
		name    string
		queued  []wire.Message
		in      wire.Message
		want    []wire.Message
		atLimit bool
	}{
		{
			name:   "same destination and path keeps latest",
			queued: []wire.Message{set(firstDest, "value=1")},
			in:     set(firstDest, "value=2"),
			want:   []wire.Message{set(firstDest, "value=2")},
		},
		{
			name:   "different path keeps earlier frame",
			queued: []wire.Message{set(firstDest, "first=1")},
			in:     set(firstDest, "second=2"),
			want:   []wire.Message{set(firstDest, "first=1")},
		},
		{
			name:   "different destination keeps earlier frame",
			queued: []wire.Message{set(firstDest, "value=1")},
			in:     set(secondDest, "value=2"),
			want:   []wire.Message{set(firstDest, "value=1")},
		},
		{
			name:   "malformed Set does not replace",
			queued: []wire.Message{set(firstDest, "value=1")},
			in:     set(firstDest, "value"),
			want:   []wire.Message{set(firstDest, "value=1")},
		},
		{
			name:   "same tag list and path",
			queued: []wire.Message{set([]any{firstDest, secondDest}, "value=1")},
			in:     set([]any{firstDest, secondDest}, "value=2"),
			want:   []wire.Message{set([]any{firstDest, secondDest}, "value=2")},
		},
		{
			name:   "replacement follows unrelated frames",
			queued: []wire.Message{set(firstDest, "value=1"), alert, set(firstDest, "other=3")},
			in:     set(firstDest, "value=2"),
			want:   []wire.Message{alert, set(firstDest, "other=3"), set(firstDest, "value=2")},
		},
		{
			name:   "replacement removes every older matching frame",
			queued: []wire.Message{set(firstDest, "value=1"), alert, set(firstDest, "value=2")},
			in:     set(firstDest, "value=3"),
			want:   []wire.Message{alert, set(firstDest, "value=3")},
		},
		{
			name:   "one-shot displaces pending Set",
			queued: []wire.Message{set(firstDest, "value=1")},
			in:     alert,
			want:   []wire.Message{alert},
		},
		{
			name:   "one-shot keeps earlier one-shot in order",
			queued: []wire.Message{set(firstDest, "value=1"), alert},
			in:     wire.Message{Dest: firstDest, What: what.Update},
			want:   []wire.Message{alert, {Dest: firstDest, What: what.Update}},
		},
		{
			name:   "nil-destination Update tick drops",
			queued: []wire.Message{set(firstDest, "value=1")},
			in:     wire.Message{What: what.Update},
			want:   []wire.Message{set(firstDest, "value=1")},
		},
		{
			name:    "one-shot overflow without Set cancels",
			queued:  []wire.Message{alert},
			in:      wire.Message{Dest: firstDest, What: what.Alert, Data: "another"},
			want:    []wire.Message{alert},
			atLimit: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queue := newBroadcastQueue(len(tc.queued))
			for _, msg := range tc.queued {
				if queue.offer(msg) {
					t.Fatal("queue overloaded before reaching capacity")
				}
			}
			if overloaded := queue.offer(tc.in); overloaded != tc.atLimit {
				t.Fatalf("offer overload = %t, want %t", overloaded, tc.atLimit)
			}
			for i, want := range tc.want {
				got, ok := queue.pop()
				if !ok || got.What != want.What || got.Data != want.Data || !sameBroadcastDest(got.Dest, want.Dest) {
					t.Fatalf("message %d = %#v, %t; want %#v", i, got, ok, want)
				}
			}
			if got, ok := queue.pop(); ok {
				t.Fatalf("extra queued message: %#v", got)
			}
		})
	}
}

func TestBroadcastQueueReadyAndClose(t *testing.T) {
	queue := newBroadcastQueue(2)
	for _, data := range []string{"value=1", "value=2"} {
		if queue.offer(wire.Message{What: what.Set, Data: data}) {
			t.Fatal("unexpected overload")
		}
	}
	for _, want := range []string{"value=1", "value=2"} {
		select {
		case <-queue.ready:
		default:
			t.Fatal("queue did not signal pending message")
		}
		if msg, ok := queue.pop(); !ok || msg.Data != want {
			t.Fatalf("pop = %#v, %t; want %q", msg, ok, want)
		}
	}
	queue.close()
	if _, ok := <-queue.ready; ok {
		t.Fatal("closed queue remained signaled")
	}
}

func TestBroadcastQueueCoalescesAfterWrap(t *testing.T) {
	queue := newBroadcastQueue(3)
	for _, data := range []string{"first=0", "second=0", "third=0"} {
		if queue.offer(wire.Message{What: what.Set, Data: data}) {
			t.Fatal("unexpected overload while filling queue")
		}
	}
	if msg, ok := queue.pop(); !ok || msg.Data != "first=0" {
		t.Fatalf("initial pop = %#v, %t", msg, ok)
	}
	if queue.offer(wire.Message{What: what.Set, Data: "first=1"}) ||
		queue.offer(wire.Message{What: what.Set, Data: "second=1"}) {
		t.Fatal("unexpected Set overload")
	}
	for _, want := range []string{"third=0", "first=1", "second=1"} {
		if msg, ok := queue.pop(); !ok || msg.Data != want {
			t.Fatalf("wrapped pop = %#v, %t; want %q", msg, ok, want)
		}
	}
}
