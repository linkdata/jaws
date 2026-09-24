package ui

import (
	"io"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/linkdata/jaws"
)

func TestJsVarStoreCoalescesBindings(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, rq := newCoreRequest(t)
		defer jw.Close()

		var mu sync.Mutex
		state := jsVarData{}
		store := NewJsVarStore(&mu, &state)
		first := store.NewJsVar()
		second := store.NewJsVar()
		if first == second || first.Ptr != second.Ptr {
			t.Fatalf("store bindings = %p, %p; want distinct bindings of one value", first, second)
		}
		firstElem := rq.NewElement(first)
		secondElem := rq.NewElement(second)
		if err := firstElem.JawsRender(io.Discard, []any{"first"}); err != nil {
			t.Fatal(err)
		}
		if err := secondElem.JawsRender(io.Discard, []any{"second"}); err != nil {
			t.Fatal(err)
		}
		if err := first.JawsSetPath(firstElem, "text", "first"); err != nil {
			t.Fatal(err)
		}
		if err := second.JawsSetPath(secondElem, "num", 7); err != nil {
			t.Fatal(err)
		}
		if err := second.JawsSetPath(secondElem, "text", "last"); err != nil {
			t.Fatal(err)
		}

		store.setMu.Lock()
		defer store.setMu.Unlock()
		if len(store.pending.updates) != 2 {
			t.Fatalf("pending paths = %d, want 2", len(store.pending.updates))
		}
		text := store.pending.updates[jsVarPendingKey{jaws: jw, path: "text"}]
		num := store.pending.updates[jsVarPendingKey{jaws: jw, path: "num"}]
		if text.msg.Data != `text="last"` || num.msg.Data != "num=7" || text.order <= num.order {
			t.Fatalf("pending text = %#v, num = %#v; want latest values in write order", text, num)
		}
	})
}

func TestJsVarStoreSeparatesJawsInstances(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		firstJaws, firstRequest := newCoreRequest(t)
		defer firstJaws.Close()
		secondJaws, secondRequest := newCoreRequest(t)
		defer secondJaws.Close()

		var mu sync.Mutex
		state := jsVarData{}
		store := NewJsVarStore(&mu, &state)
		first := store.NewJsVar()
		second := store.NewJsVar()
		firstElem := firstRequest.NewElement(first)
		secondElem := secondRequest.NewElement(second)
		if err := firstElem.JawsRender(io.Discard, []any{"first"}); err != nil {
			t.Fatal(err)
		}
		if err := secondElem.JawsRender(io.Discard, []any{"second"}); err != nil {
			t.Fatal(err)
		}
		if err := first.JawsSetPath(firstElem, "text", "first"); err != nil {
			t.Fatal(err)
		}
		if err := second.JawsSetPath(secondElem, "text", "second"); err != nil {
			t.Fatal(err)
		}

		store.setMu.Lock()
		defer store.setMu.Unlock()
		if len(store.pending.updates) != 2 {
			t.Fatalf("pending destinations = %d, want 2", len(store.pending.updates))
		}
		for jw, want := range map[*jaws.Jaws]string{firstJaws: `text="first"`, secondJaws: `text="second"`} {
			got := store.pending.updates[jsVarPendingKey{jaws: jw, path: "text"}].msg.Data
			if got != want {
				t.Fatalf("pending value for %p = %q, want %q", jw, got, want)
			}
		}
	})
}
