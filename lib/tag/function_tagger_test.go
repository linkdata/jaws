package tag

import (
	"errors"
	"reflect"
	"testing"
)

type testFunctionTagGetter func(Context) any

func (fn testFunctionTagGetter) JawsGetTag(ctx Context) any {
	return fn(ctx)
}

func TestTagExpandDoesNotConflateDistinctFunctionTagGetters(t *testing.T) {
	want := Tag("leaf")
	next := []any{want, nil}
	getters := make([]testFunctionTagGetter, len(next))
	for i := range next {
		i := i
		getters[i] = func(Context) any { return next[i] }
	}
	leafGetter := getters[0]
	rootGetter := getters[1]
	next[1] = leafGetter

	if rootPtr, leafPtr := reflect.ValueOf(rootGetter).Pointer(), reflect.ValueOf(leafGetter).Pointer(); rootPtr != leafPtr {
		t.Skipf("compiler emitted distinct code pointers %#x and %#x", rootPtr, leafPtr)
	}

	got, err := TagExpand(nil, rootGetter)
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, got, []any{want})
}

func TestTagExpandRecursiveFunctionTagGetterHitsDepthLimit(t *testing.T) {
	var recursive testFunctionTagGetter
	recursive = func(Context) any { return recursive }

	result, err := TagExpand(nil, recursive)
	if !errors.Is(err, ErrTooManyTags) {
		t.Fatalf("TagExpand() error = %v, want %v", err, ErrTooManyTags)
	}
	if result != nil {
		t.Fatalf("TagExpand() result = %#v, want nil", result)
	}
}

func TestSameActiveNodeDoesNotIdentifyFunctions(t *testing.T) {
	fn := testFunctionTagGetter(func(Context) any { return Tag("leaf") })
	if sameActiveNode(fn, fn) {
		t.Fatal("function code pointers do not identify function values")
	}
}
