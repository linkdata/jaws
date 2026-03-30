package jaws

import (
	"reflect"
	"testing"
)

type testFindTagGetter struct{}

func (testFindTagGetter) JawsGetTag(*Request) any {
	return Tag("tg")
}

type testFindTagGetterCycle struct {
	Next *testFindTagGetterCycle
}

type testFindTagGetterDepth struct {
	Next *testFindTagGetterDepth
}

func TestFindTagGetter_NoMatchCases(t *testing.T) {
	if path, tgType, found := findTagGetter(nil); found || path != "" || tgType != nil {
		t.Fatalf("expected no match for nil input, got found=%v path=%q type=%v", found, path, tgType)
	}

	cycle := &testFindTagGetterCycle{}
	cycle.Next = cycle
	shared := make([]any, 5) // length > 4 triggers bounded slice iteration path
	noMatch := struct {
		I    any
		P    *int
		Node *testFindTagGetterCycle
		Arr  [5]any
		SNil []any
		SA   []any
		SB   []any
	}{
		I:    nil, // interface nil path
		P:    nil, // pointer nil path
		Node: cycle,
		Arr:  [5]any{nil, nil, nil, nil, nil}, // length > 4 triggers bounded array iteration path
		SNil: nil,                             // nil slice path
		SA:   shared,
		SB:   shared, // same backing array triggers seen-slice path
	}

	if path, tgType, found := findTagGetter(noMatch); found || path != "" || tgType != nil {
		t.Fatalf("expected no match, got found=%v path=%q type=%v", found, path, tgType)
	}
}

func TestFindTagGetter_DepthLimit(t *testing.T) {
	root := &testFindTagGetterDepth{}
	curr := root
	for range 12 {
		curr.Next = &testFindTagGetterDepth{}
		curr = curr.Next
	}
	if path, tgType, found := findTagGetter(root); found || path != "" || tgType != nil {
		t.Fatalf("expected no match when depth limit is hit, got found=%v path=%q type=%v", found, path, tgType)
	}
}

func TestFindTagGetter_ArrayAndSliceMatches(t *testing.T) {
	withArray := struct {
		Outer struct {
			Arr [5]any
		}
	}{}
	withArray.Outer.Arr[3] = testFindTagGetter{}
	path, tgType, found := findTagGetter(withArray)
	if !found {
		t.Fatal("expected array TagGetter match")
	}
	if path != "Outer.Arr[3]" {
		t.Fatalf("unexpected array path %q", path)
	}
	if tgType != reflect.TypeFor[testFindTagGetter]() {
		t.Fatalf("unexpected array TagGetter type %v", tgType)
	}

	withSlice := struct {
		Outer struct {
			S []any
		}
	}{}
	withSlice.Outer.S = make([]any, 5)
	withSlice.Outer.S[2] = testFindTagGetter{}
	path, tgType, found = findTagGetter(withSlice)
	if !found {
		t.Fatal("expected slice TagGetter match")
	}
	if path != "Outer.S[2]" {
		t.Fatalf("unexpected slice path %q", path)
	}
	if tgType != reflect.TypeFor[testFindTagGetter]() {
		t.Fatalf("unexpected slice TagGetter type %v", tgType)
	}
}
