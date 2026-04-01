package jtag

import (
	"errors"
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/linkdata/deadlock"
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

type testTagExpandNestedTagGetter struct {
	Setter testNestedTagGetter
	Vals   []int
}

func TestTagString_StringerAndPointer(t *testing.T) {
	if got := TagString(testStringTag{}); !strings.Contains(got, "testStringTag(str)") {
		t.Fatalf("TagString(testStringTag{}) = %q, want value stringer representation", got)
	}
	if got := TagString(&testStringTag{}); !strings.Contains(got, "*jawstags.testStringTag(") {
		t.Fatalf("TagString(&testStringTag{}) = %q, want pointer representation", got)
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
			name: "error",
			tag:  boom,
			want: []any{boom},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MustTagExpand(nil, tt.tag); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MustTagExpand(%#v):\n got %#v\nwant %#v", tt.tag, got, tt.want)
			}
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

func TestTagExpand_TooManyTagsPanic(t *testing.T) {
	tags := []any{nil}
	tags[0] = tags // infinite recursion in the tags expansion
	defer func() {
		x := recover()
		e, ok := x.(error)
		if !ok {
			t.Fail()
		}
		if e.Error() != ErrTooManyTags.Error() {
			t.Fail()
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
	if !strings.Contains(err.Error(), "implement JawsGetTag(jawstags.Context)") {
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
	err error
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
	want := []any{Tag("nested")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TagExpand(testNestedTagGetter{}):\n got %#v\nwant %#v", got, want)
	}
}

func TestMustTagExpand_UsesContextMustLog(t *testing.T) {
	ctx := &mustLogContext{}
	got := MustTagExpand(ctx, "plain-string")
	if got != nil {
		t.Fatalf("MustTagExpand returned %#v, want nil", got)
	}
	if !errors.Is(ctx.err, ErrIllegalTagType) {
		t.Fatalf("expected ErrIllegalTagType, got %v", ctx.err)
	}
}
