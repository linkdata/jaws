package tag

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/key"
)

type testSelfTagger struct{}

func (tt *testSelfTagger) JawsGetTag(Context) any {
	return tt
}

type testBadTagGetter []int

func (tt testBadTagGetter) JawsGetTag(Context) any {
	return tt
}

type testStringTag struct{}

func (testStringTag) String() string { return "str" }

type testNestedTagGetter struct{}

func (testNestedTagGetter) JawsGetTag(Context) any {
	return Tag("nested")
}

type testSelfSliceTagger struct{}

func (tt *testSelfSliceTagger) JawsGetTag(Context) any {
	return []any{tt}
}

type testSelfSliceExtraTagger struct{}

func (tt *testSelfSliceExtraTagger) JawsGetTag(Context) any {
	return []any{tt, Tag("extra")}
}

type testMutualSliceTagger struct {
	next *testMutualSliceTagger
	name string
}

func (tt *testMutualSliceTagger) JawsGetTag(Context) any {
	return []any{tt.next}
}

type testTagExpandNestedTagGetter struct {
	Setter testNestedTagGetter
	Vals   []int
}

type testNonComparableActiveNode struct {
	Values []int
}

type testDeepTagGetter struct {
	next any
}

func (tt testDeepTagGetter) JawsGetTag(Context) any {
	return tt.next
}

func TestTagString_StringerAndPointer(t *testing.T) {
	if got := TagString(testStringTag{}); !strings.Contains(got, "testStringTag(str)") {
		t.Fatalf("TagString(testStringTag{}) = %q, want value stringer representation", got)
	}
	if got := TagString(&testStringTag{}); !strings.Contains(got, "*tag.testStringTag(") {
		t.Fatalf("TagString(&testStringTag{}) = %q, want pointer representation", got)
	}
}

func assertTagSetEqual(t *testing.T, got, want []any) {
	t.Helper()
	gotSet := make(map[any]struct{}, len(got))
	for _, v := range got {
		gotSet[v] = struct{}{}
	}
	wantSet := make(map[any]struct{}, len(want))
	for _, v := range want {
		wantSet[v] = struct{}{}
	}
	if !reflect.DeepEqual(gotSet, wantSet) {
		t.Fatalf("tag set mismatch:\n got %#v\nwant %#v", got, want)
	}
}

func TestTagExpand(t *testing.T) {
	var av atomic.Value
	selftagger := &testSelfTagger{}
	boom := errors.New("boom")
	tests := []struct {
		name string
		tag  any
		want []any
	}{
		{
			name: "nil",
			tag:  nil,
			want: nil,
		},
		{
			name: "Tag",
			tag:  Tag("foo"),
			want: []any{Tag("foo")},
		},
		{
			name: "TagGetter(Self)",
			tag:  selftagger,
			want: []any{selftagger},
		},
		{
			name: "[]Tag",
			tag:  []Tag{Tag("a"), Tag("b"), Tag("c")},
			want: []any{Tag("a"), Tag("b"), Tag("c")},
		},
		{
			name: "[]any",
			tag:  []any{Tag("a"), &av},
			want: []any{Tag("a"), &av},
		},
		{
			name: "[]any nils",
			tag:  []any{nil, nil},
			want: nil,
		},
		{
			name: "error",
			tag:  boom,
			want: []any{boom},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertTagSetEqual(t, MustTagExpand(nil, tt.tag), tt.want)
		})
	}
}

func TestTagExpand_IllegalTypesPanic(t *testing.T) {
	if !deadlock.Debug {
		t.Log("skipped, not debugging")
		return
	}
	tags := []any{
		string("string"),
		template.HTML("template.HTML"),
		template.HTMLAttr("template.HTMLAttr"),
		int(1),
		int8(2),
		int16(3),
		int32(4),
		int64(5),
		uint(6),
		uint8(7),
		uint16(8),
		uint32(9),
		uint64(10),
		key.Key(11),
		float32(11),
		float64(12),
		bool(true),
		[]string{"a", "b"},
		[]template.HTML{"a", "b"},
		map[int]int{1: 1},
	}
	for _, tag := range tags {
		t.Run(fmt.Sprintf("%T", tag), func(t *testing.T) {
			defer func() {
				x := recover()
				e, ok := x.(error)
				if !ok {
					t.FailNow()
				}
				if !(errors.Is(e, ErrIllegalTagType) || errors.Is(e, ErrNotComparable)) {
					t.FailNow()
				}
				if !strings.Contains(e.Error(), fmt.Sprintf("%T", tag)) {
					t.FailNow()
				}
			}()
			MustTagExpand(nil, tag)
			t.FailNow()
		})
	}
}

func TestTagExpand_SelfReferentialSliceStopsRecursing(t *testing.T) {
	tags := []any{nil}
	tags[0] = tags
	got, err := TagExpand(nil, tags)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no tags, got %#v", got)
	}
}

func TestTagExpand_TooManyTagsPanic(t *testing.T) {
	tags := make([]any, 101)
	for i := range tags {
		tags[i] = Tag(fmt.Sprintf("t%d", i))
	}
	defer func() {
		x := recover()
		e, ok := x.(error)
		if !ok {
			t.Fatal("expected error, got", x)
		}
		if !errors.Is(e, ErrTooManyTags) {
			t.Errorf("recovered error = %v, want %v", e, ErrTooManyTags)
		}
		if e.Error() != "too many tags" {
			t.Errorf("ErrTooManyTags.Error() = %q", e.Error())
		}
	}()
	MustTagExpand(nil, tags)
	t.FailNow()
}

func TestTagExpand_TagGetterNonComparable(t *testing.T) {
	_, err := TagExpand(nil, testBadTagGetter{1})
	if !errors.Is(err, ErrNotUsableAsTag) {
		t.Fatalf("expected ErrNotUsableAsTag, got %v", err)
	}
	if !errors.Is(err, ErrNotComparable) {
		t.Fatalf("expected ErrNotComparable, got %v", err)
	}
	if !strings.Contains(err.Error(), "found nested TagGetter at <value>") {
		t.Fatalf("expected TagGetter search result in error text, got %q", err.Error())
	}
}

// TestTagExpand_RuntimeNonComparable covers the gap between static and runtime
// comparability: a struct whose static type is comparable but that holds a
// non-comparable value in an interface field (here a func) passes
// reflect.Type.Comparable() yet panics on == / as a map key. ensureUsableTag runs
// the runtime comparability check for struct and array kinds in all builds, so tag
// expansion must reject even a single such tag with ErrNotUsableAsTag rather than
// accepting it and deferring a panic to jw.dirty / rq.tagMap on the event
// goroutine.
func TestTagExpand_RuntimeNonComparable(t *testing.T) {
	if _, err := TagExpand(nil, testRuntimeNonComparable{v: func() {}}); !errors.Is(err, ErrNotUsableAsTag) {
		t.Fatalf("expected ErrNotUsableAsTag, got %v", err)
	}
}

// TestTagExpand_MultiRuntimeNonComparable covers the multi-element case: two
// same-typed runtime-non-comparable values in one expansion. ensureUsableTag
// rejects the first one with ErrNotUsableAsTag before the dedup existing == tag in
// appendUniqueTag ever compares them; it must not panic and must report
// ErrNotUsableAsTag with no tags.
func TestTagExpand_MultiRuntimeNonComparable(t *testing.T) {
	a := testRuntimeNonComparable{v: func() {}}
	b := testRuntimeNonComparable{v: func() {}}
	result, err := TagExpand(nil, []any{a, b})
	if !errors.Is(err, ErrNotUsableAsTag) {
		t.Fatalf("expected ErrNotUsableAsTag, got %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected no tags, got %v", result)
	}
}

// TestTagExpand_RepanicsOtherPanics ensures TagExpand's recover only intercepts the
// comparability panic: a panic from a [TagGetter] callback must propagate untouched
// so unrelated bugs are not masked.
func TestTagExpand_RepanicsOtherPanics(t *testing.T) {
	defer func() {
		switch r := recover().(type) {
		case nil:
			t.Fatal("expected panic to propagate")
		case string:
			if r != "boom" {
				t.Fatalf("unexpected panic value %v", r)
			}
		default:
			t.Fatalf("unexpected panic value %v", r)
		}
	}()
	_, _ = TagExpand(nil, testPanicTagGetter{})
}

type testPanicTagGetter struct{}

func (testPanicTagGetter) JawsGetTag(Context) any { panic("boom") }

// uncomparablePanic returns a real "comparing uncomparable type" runtime panic
// value by comparing two non-comparable interface values.
func uncomparablePanic() (r any) {
	defer func() { r = recover() }()
	var a, b any = []int{1}, []int{1}
	_ = a == b
	return
}

// Test_recoverComparabilityPanic exercises the recovery helper directly, since the
// comparability panic it handles only occurs in production builds (debug builds
// reject such tags in ensureUsableTag before the dedup compares them).
func Test_recoverComparabilityPanic(t *testing.T) {
	rerr := uncomparablePanic()
	if _, ok := rerr.(runtime.Error); !ok {
		t.Fatalf("expected a runtime.Error, got %T (%v)", rerr, rerr)
	}

	// Non-comparable top-level tag: NewErrNotUsableAsTag returns a non-nil error.
	if result, err := recoverComparabilityPanic(rerr, []int{1}); result != nil || !errors.Is(err, ErrNotUsableAsTag) {
		t.Fatalf("non-comparable tag: result=%v err=%v", result, err)
	}

	// Comparable top-level tag (a nested element panicked): falls back to the
	// ErrNotUsableAsTag sentinel.
	if result, err := recoverComparabilityPanic(rerr, Tag("x")); result != nil || !errors.Is(err, ErrNotUsableAsTag) {
		t.Fatalf("comparable tag: result=%v err=%v", result, err)
	}

	// Any other panic value is re-raised unchanged.
	func() {
		defer func() {
			if r := recover(); r != "boom" {
				t.Fatalf("expected re-raised \"boom\", got %v", r)
			}
		}()
		_, _ = recoverComparabilityPanic("boom", nil)
	}()
}

func TestTagExpand_NotUsableAsTag_WithNestedTagGetterHint(t *testing.T) {
	tag := testTagExpandNestedTagGetter{
		Setter: testNestedTagGetter{},
		Vals:   []int{1},
	}
	_, err := TagExpand(nil, tag)
	if !errors.Is(err, ErrNotUsableAsTag) {
		t.Fatalf("expected ErrNotUsableAsTag, got %v", err)
	}
	if !errors.Is(err, ErrNotComparable) {
		t.Fatalf("expected ErrNotComparable compatibility, got %v", err)
	}
	if !strings.Contains(err.Error(), "found nested TagGetter at Setter") {
		t.Fatalf("expected nested TagGetter search result in error text, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "implement JawsGetTag(tag.Context)") {
		t.Fatalf("expected remediation hint in error text, got %q", err.Error())
	}
}

func TestTagExpand_NotUsableAsTag_NoNestedTagGetterHint(t *testing.T) {
	_, err := TagExpand(nil, map[int]int{1: 1})
	if !errors.Is(err, ErrNotUsableAsTag) {
		t.Fatalf("expected ErrNotUsableAsTag, got %v", err)
	}
	if !errors.Is(err, ErrNotComparable) {
		t.Fatalf("expected ErrNotComparable compatibility, got %v", err)
	}
	if !strings.Contains(err.Error(), "found no nested TagGetter") {
		t.Fatalf("expected no-TagGetter search result in error text, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "use a comparable tag value") {
		t.Fatalf("expected remediation hint in error text, got %q", err.Error())
	}
}

func TestTagExpand_IllegalTagTypeError(t *testing.T) {
	_, err := TagExpand(nil, "plain-string")
	if !errors.Is(err, ErrIllegalTagType) {
		t.Fatalf("expected ErrIllegalTagType, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "string") {
		t.Fatalf("expected error text to contain type name, got %v", err)
	}
}

func TestTagExpand_IllegalTypesAsErrors(t *testing.T) {
	tests := []struct {
		name    string
		tag     any
		wantErr error
	}{
		{name: "template.HTML", tag: template.HTML("x"), wantErr: ErrIllegalTagType},
		{name: "template.HTMLAttr", tag: template.HTMLAttr(`x="y"`), wantErr: ErrIllegalTagType},
		{name: "int", tag: int(1), wantErr: ErrIllegalTagType},
		{name: "int8", tag: int8(2), wantErr: ErrIllegalTagType},
		{name: "int16", tag: int16(3), wantErr: ErrIllegalTagType},
		{name: "int32", tag: int32(4), wantErr: ErrIllegalTagType},
		{name: "int64", tag: int64(5), wantErr: ErrIllegalTagType},
		{name: "uint", tag: uint(6), wantErr: ErrIllegalTagType},
		{name: "uint8", tag: uint8(7), wantErr: ErrIllegalTagType},
		{name: "uint16", tag: uint16(8), wantErr: ErrIllegalTagType},
		{name: "uint32", tag: uint32(9), wantErr: ErrIllegalTagType},
		{name: "uint64", tag: uint64(10), wantErr: ErrIllegalTagType},
		{name: "key.Key", tag: key.Key(11), wantErr: ErrIllegalTagType},
		{name: "float32", tag: float32(1), wantErr: ErrIllegalTagType},
		{name: "float64", tag: float64(2), wantErr: ErrIllegalTagType},
		{name: "bool", tag: true, wantErr: ErrIllegalTagType},
		{name: "map", tag: map[int]int{1: 1}, wantErr: ErrNotComparable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := TagExpand(nil, tt.tag)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("TagExpand(%T): got %v want %v", tt.tag, err, tt.wantErr)
			}
		})
	}
}

type mustLogContext struct {
	ctx context.Context
	err error
}

func (ctx *mustLogContext) Initial() *http.Request {
	return nil
}

func (ctx *mustLogContext) Get(string) any {
	return nil
}

func (ctx *mustLogContext) Set(key string, value any) {}

func (ctx *mustLogContext) Context() context.Context {
	return ctx.ctx
}

func (ctx *mustLogContext) Log(err error) error {
	ctx.err = err
	return err
}

func (ctx *mustLogContext) MustLog(err error) {
	ctx.err = err
}

func TestTagString_DefaultFormatting(t *testing.T) {
	got := TagString(Tag("plain"))
	if !strings.Contains(got, `"plain"`) {
		t.Fatalf("TagString(Tag(\"plain\")) = %q, want quoted fallback formatting", got)
	}
}

func TestTagExpand_TagGetterRecurses(t *testing.T) {
	got, err := TagExpand(nil, testNestedTagGetter{})
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, got, []any{Tag("nested")})
}

func TestTagExpand_TagGetterSelfInSlice(t *testing.T) {
	self := &testSelfSliceTagger{}
	got, err := TagExpand(nil, self)
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, got, []any{self})
}

func TestTagExpand_TagGetterSelfAndExtraInSlice(t *testing.T) {
	self := &testSelfSliceExtraTagger{}
	got, err := TagExpand(nil, self)
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, got, []any{self, Tag("extra")})
}

func TestTagExpand_TagGetterMutualCycleExpandsToCycleMembers(t *testing.T) {
	a := &testMutualSliceTagger{name: "a"}
	b := &testMutualSliceTagger{name: "b", next: a}
	a.next = b
	got, err := TagExpand(nil, a)
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, got, []any{a, b})
}

func TestMustTagExpand_UsesContextMustLog(t *testing.T) {
	ctx := &mustLogContext{ctx: t.Context()}
	got := MustTagExpand(ctx, "plain-string")
	if got != nil {
		t.Fatalf("MustTagExpand returned %#v, want nil", got)
	}
	if !errors.Is(ctx.err, ErrIllegalTagType) {
		t.Fatalf("expected ErrIllegalTagType, got %v", ctx.err)
	}
}

func TestNewErrNotComparable_ComparableAndNil(t *testing.T) {
	if err := NewErrNotComparable(Tag("ok")); err != nil {
		t.Fatalf("NewErrNotComparable(comparable) = %v, want nil", err)
	}
	if err := NewErrNotComparable(nil); err != nil {
		t.Fatalf("NewErrNotComparable(nil) = %v, want nil", err)
	}
}

func TestNewErrNotUsableAsTag_ComparableAndNil(t *testing.T) {
	if err := NewErrNotUsableAsTag(Tag("ok")); err != nil {
		t.Fatalf("NewErrNotUsableAsTag(comparable) = %v, want nil", err)
	}
	if err := NewErrNotUsableAsTag(nil); err != nil {
		t.Fatalf("NewErrNotUsableAsTag(nil) = %v, want nil", err)
	}
}

func TestAppendUniqueTag_Deduplicates(t *testing.T) {
	got, err := appendUniqueTag([]any{Tag("dup")}, Tag("dup"))
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, got, []any{Tag("dup")})
}

func TestSameActiveNode_NilAndDefaultCases(t *testing.T) {
	if !sameActiveNode(nil, nil) {
		t.Fatal("expected nil nodes to match")
	}
	if sameActiveNode(nil, Tag("x")) {
		t.Fatal("expected nil and non-nil nodes not to match")
	}
	a := testNonComparableActiveNode{Values: []int{1}}
	b := testNonComparableActiveNode{Values: []int{1}}
	if sameActiveNode(a, b) {
		t.Fatal("expected non-comparable structs to compare by identity, not contents")
	}
}

func TestTagExpand_TooDeepAndTooManySliceTags(t *testing.T) {
	var nested any = Tag("leaf")
	for range 11 {
		nested = testDeepTagGetter{next: nested}
	}
	if _, err := TagExpand(nil, nested); !errors.Is(err, ErrTooManyTags) {
		t.Fatalf("TagExpand(deep) = %v, want %v", err, ErrTooManyTags)
	}

	tags := make([]Tag, 101)
	for i := range tags {
		tags[i] = Tag(fmt.Sprintf("t%d", i))
	}
	if _, err := TagExpand(nil, tags); !errors.Is(err, ErrTooManyTags) {
		t.Fatalf("TagExpand([]Tag) = %v, want %v", err, ErrTooManyTags)
	}
}
