package core

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

type testSelfTagger struct {
}

func (tt *testSelfTagger) JawsGetTag(rq *Request) any {
	return tt
}

type testBadTagGetter []int

func (tt testBadTagGetter) JawsGetTag(*Request) any {
	return tt
}

func TestTagExpand(t *testing.T) {
	var av atomic.Value
	selftagger := &testSelfTagger{}
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
			tag:  ErrEventUnhandled,
			want: []any{ErrEventUnhandled},
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
	if !errors.Is(err, ErrNotComparable) {
		t.Fatalf("expected ErrNotComparable, got %v", err)
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
