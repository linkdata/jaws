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
	"github.com/linkdata/jaws/lib/jid"
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

func TestTagStringDebug_StringerAndPointer(t *testing.T) {
	if got := TagStringDebug(testStringTag{}); !strings.Contains(got, "testStringTag(str)") {
		t.Fatalf("TagStringDebug(testStringTag{}) = %q, want value stringer representation", got)
	}
	if got := TagStringDebug(&testStringTag{}); !strings.Contains(got, "*tag.testStringTag(") {
		t.Fatalf("TagStringDebug(&testStringTag{}) = %q, want pointer representation", got)
	}
	if got := TagStringDebug(Tag("plain")); !strings.Contains(got, `"plain"`) {
		t.Fatalf("TagStringDebug(Tag(\"plain\")) = %q, want quoted value", got)
	}
}

func TestTagStringRelease_TypeAndAddress(t *testing.T) {
	// Release rendering shows the type — plus a pointer's address when it can be
	// read safely — but never the value or a method result, so it cannot descend.
	if got, want := TagStringRelease(testStringTag{}), "tag.testStringTag"; got != want {
		t.Fatalf("TagStringRelease(testStringTag{}) = %q, want %q", got, want)
	}
	if got := TagStringRelease(&testStringTag{}); !strings.HasPrefix(got, "*tag.testStringTag(0x") {
		t.Fatalf("TagStringRelease(&testStringTag{}) = %q, want type and address", got)
	}
	if got, want := TagStringRelease(Tag("plain")), "tag.Tag"; got != want {
		t.Fatalf("TagStringRelease(Tag(\"plain\")) = %q, want %q", got, want)
	}
	if got, want := TagStringRelease(nil), "<nil>"; got != want {
		t.Fatalf("TagStringRelease(nil) = %q, want %q", got, want)
	}
}

func TestPointerAddr_RecoversPanic(t *testing.T) {
	// reflect.Value.Pointer panics on a non-pointer kind (and on some cgo /
	// not-in-heap pointers); pointerAddr must recover and report failure so release
	// rendering degrades to type-only instead of crashing.
	if _, ok := pointerAddr(reflect.ValueOf(42)); ok {
		t.Error("pointerAddr(non-pointer) reported ok, want a recovered failure")
	}
	x := 0
	if addr, ok := pointerAddr(reflect.ValueOf(&x)); !ok || addr == 0 {
		t.Errorf("pointerAddr(&x) = (%#x, %v), want a non-zero address and ok", addr, ok)
	}
}

func TestTagStringRelease_SafeOnCyclic(t *testing.T) {
	// The release renderer must stay bounded on values that overflow fmt. This runs
	// in every build (including -race) because it never descends into the value.
	cyclic := []any{nil}
	cyclic[0] = cyclic
	if got, want := TagStringRelease(cyclic), "[]interface {}"; got != want {
		t.Errorf("TagStringRelease(cyclic slice) = %q, want %q", got, want)
	}
	if got, want := TagsStringRelease(cyclic), "[[]interface {}]"; got != want {
		t.Errorf("TagsStringRelease(cyclic slice) = %q, want %q", got, want)
	}
	m := map[int]any{}
	m[0] = m
	if got, want := TagStringRelease(m), "map[int]interface {}"; got != want {
		t.Errorf("TagStringRelease(cyclic map) = %q, want %q", got, want)
	}
}

// panicOnRender's formatting methods all panic if called.
//
// Invoking one directly crashes; invoking one via fmt yields a recovered
// "%!v(PANIC=...)" marker. A renderer that produces neither — just the plain
// type — therefore never called them.
type panicOnRender struct{}

func (panicOnRender) String() string         { panic("String called") }
func (panicOnRender) GoString() string       { panic("GoString called") }
func (panicOnRender) Format(fmt.State, rune) { panic("Format called") }

func TestTagStringRelease_NeverInvokesMethods(t *testing.T) {
	// The release path reads only the type and address, so a tag whose String,
	// GoString or Format methods panic (or would recurse) still renders safely.
	for _, v := range []any{panicOnRender{}, &panicOnRender{}} {
		got := TagStringRelease(v)
		if strings.Contains(got, "PANIC") || strings.Contains(got, "called") {
			t.Errorf("TagStringRelease(%T) = %q, want no method invocation", v, got)
		}
		if !strings.Contains(got, "panicOnRender") {
			t.Errorf("TagStringRelease(%T) = %q, want the type name", v, got)
		}
	}
	if got := TagsStringRelease([]any{panicOnRender{}}); !strings.Contains(got, "panicOnRender") || strings.Contains(got, "PANIC") {
		t.Errorf("TagsStringRelease = %q, want type name and no panic", got)
	}
}

func TestTagStringRelease_BoundsHugeTypeName(t *testing.T) {
	// A valid comparable tag can still have a pathologically long type name (here
	// via deeply nested arrays). Release rendering must truncate it so the output
	// stays bounded rather than proportional to the type name.
	typ := reflect.TypeOf(byte(0))
	for range 5000 {
		typ = reflect.ArrayOf(1, typ)
	}
	v := reflect.New(typ).Elem().Interface()
	got := TagStringRelease(v)
	if len(got) > maxTagString+len(truncMarker) {
		t.Errorf("TagStringRelease(huge type) = %d bytes, want <= %d", len(got), maxTagString+len(truncMarker))
	}
	if !strings.HasSuffix(got, truncMarker) {
		t.Errorf("truncated render should end with %q, got …%q", truncMarker, got[max(0, len(got)-8):])
	}
}

func TestClipTagString(t *testing.T) {
	if got := clipTagString("short"); got != "short" {
		t.Errorf("clipTagString(short) = %q, want it unchanged", got)
	}
	// Place a two-byte rune straddling the cap so truncation must back up to the
	// rune boundary rather than split it.
	s := strings.Repeat("a", maxTagString-1) + "é" + "tail"
	got := clipTagString(s)
	if len(got) > maxTagString+len(truncMarker) {
		t.Errorf("clipTagString len = %d, want <= %d", len(got), maxTagString+len(truncMarker))
	}
	if !strings.HasSuffix(got, truncMarker) {
		t.Errorf("clipTagString should end with %q, got …%q", truncMarker, got[len(got)-4:])
	}
	if strings.ToValidUTF8(got, "�") != got {
		t.Error("clipTagString split a multi-byte rune (invalid UTF-8)")
	}
}

func TestTagsStringRelease_BoundsHugeList(t *testing.T) {
	// An enormous tag list must render to a bounded string, not one proportional to
	// the slice length.
	tags := make([]any, 100_000)
	for i := range tags {
		tags[i] = Tag("x")
	}
	if got := TagsStringRelease(tags); len(got) > maxTagString+64 {
		t.Errorf("TagsStringRelease(100k tags) = %d bytes, want bounded near %d", len(got), maxTagString)
	}
}

func TestTagsStringDebug_RendersElements(t *testing.T) {
	// Debug rendering of a slice quotes each element via TagStringDebug.
	if got, want := TagsStringDebug([]any{Tag("a"), Tag("b")}), `["a" "b"]`; got != want {
		t.Errorf("TagsStringDebug = %q, want %q", got, want)
	}
}

func TestTagString_ForwardsByBuild(t *testing.T) {
	// TagString/TagsString forward to the release or debug variant per build. Verify
	// the routing with a safe value (a cyclic one would crash the debug forwarder).
	v := Tag("x")
	want := TagStringRelease(v)
	if DebugRender {
		want = TagStringDebug(v)
	}
	if got := TagString(v); got != want {
		t.Errorf("TagString(%v) = %q, want %q (DebugRender=%v)", v, got, want, DebugRender)
	}
	tags := []any{Tag("a"), Tag("b")}
	twant := TagsStringRelease(tags)
	if DebugRender {
		twant = TagsStringDebug(tags)
	}
	if got := TagsString(tags); got != twant {
		t.Errorf("TagsString = %q, want %q", got, twant)
	}
}

func assertTagSetEqual(t *testing.T, got, want []any) {
	t.Helper()
	// Compare as sets so iteration order is not asserted, but also require equal
	// lengths so a duplicate in got cannot be silently collapsed into the set.
	// Every want passed here is duplicate-free, so equal length plus equal set
	// means got is a duplicate-free permutation of want. The length check is what
	// makes deduplication (the package's core guarantee) actually testable:
	// without it, a broken expansion emitting [dup, dup] would still pass.
	gotSet := make(map[any]struct{}, len(got))
	for _, v := range got {
		gotSet[v] = struct{}{}
	}
	wantSet := make(map[any]struct{}, len(want))
	for _, v := range want {
		wantSet[v] = struct{}{}
	}
	if len(got) != len(want) || !reflect.DeepEqual(gotSet, wantSet) {
		t.Fatalf("tag set mismatch:\n got %#v (len %d)\nwant %#v (len %d)", got, len(got), want, len(want))
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

// TestTagExpand_AliasedSliceViews reproduces #203: two slices sharing a backing
// array at the same start but with different lengths must not be conflated as a
// cycle. Expanding the shorter view whose element references the longer backing
// slice must still visit every tag reachable through the longer slice.
func TestTagExpand_AliasedSliceViews(t *testing.T) {
	all := make([]any, 3)
	outer := all[:2]

	all[0] = Tag("first")
	all[1] = all
	all[2] = Tag("last")

	got, err := TagExpand(nil, outer)
	if err != nil {
		t.Fatal(err)
	}
	want := []any{Tag("first"), Tag("last")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TagExpand() = %#v, want %#v", got, want)
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
// non-comparable value in an interface field (here a func) panics on == or as a
// map key. Tag expansion must reject even a single such tag with
// ErrNotUsableAsTag rather than deferring that panic to jw.dirty or rq.tagMap.
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

// TestTagExpand_ValidThenRuntimeNonComparable covers a valid tag preceding a
// runtime-non-comparable one: TagExpand must honor its nil-result contract for
// ErrNotUsableAsTag and not leak the partial result accumulated before the
// rejection.
func TestTagExpand_ValidThenRuntimeNonComparable(t *testing.T) {
	result, err := TagExpand(nil, []any{Tag("a"), testRuntimeNonComparable{v: func() {}}})
	if !errors.Is(err, ErrNotUsableAsTag) {
		t.Fatalf("expected ErrNotUsableAsTag, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}

// TestTagExpand_RuntimeNonComparableArray covers an array whose static type is
// comparable ([1]any) but whose element holds a non-comparable value (a func).
// Comparing it panics, so expansion must reject it with ErrNotUsableAsTag.
func TestTagExpand_RuntimeNonComparableArray(t *testing.T) {
	if _, err := TagExpand(nil, [1]any{func() {}}); !errors.Is(err, ErrNotUsableAsTag) {
		t.Fatalf("expected ErrNotUsableAsTag, got %v", err)
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
		{name: "jid.Jid", tag: jid.Jid(11), wantErr: ErrIllegalTagType},
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

func TestSameActiveNode_AliasedSliceViews(t *testing.T) {
	backing := []any{Tag("a"), Tag("b"), Tag("c")}
	short := backing[:2]
	// Same start pointer, different lengths: distinct traversal nodes (#203).
	if sameActiveNode(short, backing) {
		t.Fatal("expected aliased slice views of different lengths not to match")
	}
	// Identical start pointer and length: the same node, so a genuine
	// self-referential slice is still detected as a cycle.
	if !sameActiveNode(backing, backing) {
		t.Fatal("expected a slice to match itself")
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

// TestTagExpand_PartialResultOnCountLimit pins the documented contract that on
// the count-limit path TagExpand returns the tags expanded before the failure.
// The cap is inclusive (the over-limit tag is appended before the check), so the
// partial result holds maxTagCount+1 elements in input order.
func TestTagExpand_PartialResultOnCountLimit(t *testing.T) {
	tags := make([]Tag, maxTagCount+1)
	for i := range tags {
		tags[i] = Tag(fmt.Sprintf("t%d", i))
	}
	result, err := TagExpand(nil, tags)
	if !errors.Is(err, ErrTooManyTags) {
		t.Fatalf("TagExpand([]Tag) error = %v, want %v", err, ErrTooManyTags)
	}
	if len(result) != maxTagCount+1 {
		t.Fatalf("partial result len = %d, want %d", len(result), maxTagCount+1)
	}
	for i, got := range result {
		if want := Tag(fmt.Sprintf("t%d", i)); got != want {
			t.Errorf("result[%d] = %v, want %v", i, got, want)
		}
	}
}
