package jaws

import (
	"reflect"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestSetBatchKeepsLastValueInLastWriteOrder(t *testing.T) {
	var batch setBatch
	for _, msg := range []wire.Message{
		{Dest: tag.Tag("state"), What: what.Set, Data: "root=1"},
		{Dest: tag.Tag("state"), What: what.Set, Data: "child=2"},
		{Dest: tag.Tag("other"), What: what.Set, Data: "child=3"},
		{Dest: tag.Tag("state"), What: what.Set, Data: "root=4"},
	} {
		if !batch.add(msg) {
			t.Fatalf("valid Set was not batched: %#v", msg)
		}
	}
	got := batch.take()
	want := setGroup{
		{Dest: tag.Tag("state"), What: what.Set, Data: "child=2"},
		{Dest: tag.Tag("other"), What: what.Set, Data: "child=3"},
		{Dest: tag.Tag("state"), What: what.Set, Data: "root=4"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("batched Sets = %#v, want %#v", got, want)
	}
	if batch.add(wire.Message{Dest: tag.Tag("state"), What: what.Set, Data: "missing-equals"}) {
		t.Fatal("malformed Set was batched")
	}
	got = batch.take()
	if len(got) != 0 {
		t.Fatalf("second flush = %#v, want no messages", got)
	}
}

func TestSetBatchMatchesMultiTagDestinationsByIdentity(t *testing.T) {
	var batch setBatch
	a, b := new(int), new(int)
	for _, msg := range []wire.Message{
		{Dest: []any{a, tag.Tag("state")}, What: what.Set, Data: "x=1"},
		{Dest: []any{b, tag.Tag("state")}, What: what.Set, Data: "x=2"},
		{Dest: []any{tag.Tag("state"), a}, What: what.Set, Data: "x=3"},
	} {
		if !batch.add(msg) {
			t.Fatalf("valid Set was not batched: %#v", msg)
		}
	}
	got := batch.take()
	want := setGroup{
		{Dest: []any{b, tag.Tag("state")}, What: what.Set, Data: "x=2"},
		{Dest: []any{tag.Tag("state"), a}, What: what.Set, Data: "x=3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("batched multi-tag Sets = %#v, want %#v", got, want)
	}
}
