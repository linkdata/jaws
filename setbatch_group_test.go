package jaws

import (
	"reflect"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestSetBatchGroupsOnlyAdjacentDestinations(t *testing.T) {
	var batch setBatch
	a := tag.Tag("left")
	b := tag.Tag("shared")
	// A and B may select the same browser value; the B root write must remain
	// between the two A groups.
	for _, msg := range []wire.Message{
		{Dest: a, What: what.Set, Data: "x=1"},
		{Dest: a, What: what.Set, Data: "z=1"},
		{Dest: b, What: what.Set, Data: `={"x":2,"z":2}`},
		{Dest: a, What: what.Set, Data: "y=3"},
	} {
		if !batch.add(msg) {
			t.Fatalf("valid Set was not batched: %#v", msg)
		}
	}

	var got []wire.Message
	batch.flush(func(msg wire.Message) { got = append(got, msg) })
	want := []wire.Message{
		{Dest: a, What: what.Set, Data: "x=1\nz=1"},
		{Dest: b, What: what.Set, Data: `={"x":2,"z":2}`},
		{Dest: a, What: what.Set, Data: "y=3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("grouped Sets = %#v, want %#v", got, want)
	}
}

func TestSetBatchGroupsEquivalentMultiTagDestinations(t *testing.T) {
	var batch setBatch
	first := []any{tag.Tag("left"), tag.Tag("shared")}
	second := []any{tag.Tag("shared"), tag.Tag("left")}
	for _, msg := range []wire.Message{
		{Dest: first, What: what.Set, Data: "x=1"},
		{Dest: second, What: what.Set, Data: "y=2"},
	} {
		if !batch.add(msg) {
			t.Fatalf("valid Set was not batched: %#v", msg)
		}
	}

	var got []wire.Message
	batch.flush(func(msg wire.Message) { got = append(got, msg) })
	if len(got) != 1 || got[0].What != what.Set || got[0].Data != "x=1\ny=2" {
		t.Fatalf("grouped multi-tag Sets = %#v, want one ordered group", got)
	}
	if dest, ok := got[0].Dest.([]any); !ok || !sameSetDest(dest, first) {
		t.Fatalf("grouped destination = %#v, want tags %#v", got[0].Dest, first)
	}
}
