package tag

import (
	"errors"
	"reflect"
	"testing"
)

type testFunctionTagGetter func() any

func (fn testFunctionTagGetter) JawsGetTag() any {
	return fn()
}

func TestTagExpandDoesNotConflateDistinctFunctionTagGetters(t *testing.T) {
	want := Tag("leaf")
	next := []any{want, nil}
	getters := make([]testFunctionTagGetter, len(next))
	for i := range next {
		i := i
		getters[i] = func() any { return next[i] }
	}
	leafGetter := getters[0]
	rootGetter := getters[1]
	next[1] = leafGetter

	if rootPtr, leafPtr := reflect.ValueOf(rootGetter).Pointer(), reflect.ValueOf(leafGetter).Pointer(); rootPtr != leafPtr {
		t.Skipf("compiler emitted distinct code pointers %#x and %#x", rootPtr, leafPtr)
	}

	got, err := TagExpand(rootGetter)
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, got, []any{want})
}

func TestTagExpandRecursiveFunctionTagGetterHitsDepthLimit(t *testing.T) {
	var recursive testFunctionTagGetter
	recursive = func() any { return recursive }

	result, err := TagExpand(recursive)
	if !errors.Is(err, ErrTooManyTags) {
		t.Fatalf("TagExpand() error = %v, want %v", err, ErrTooManyTags)
	}
	if result != nil {
		t.Fatalf("TagExpand() result = %#v, want nil", result)
	}
}

func TestSameActiveNodeDoesNotIdentifyFunctions(t *testing.T) {
	fn := testFunctionTagGetter(func() any { return Tag("leaf") })
	if sameActiveNode(fn, fn) {
		t.Fatal("function code pointers do not identify function values")
	}
}
