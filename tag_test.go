package jaws

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

func TestTagExpand(t *testing.T) {
	var av atomic.Value
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
			name: "TagGetter",
			tag:  atomicSetter{&av},
			want: []any{&av},
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
		errors.New("error"),
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
