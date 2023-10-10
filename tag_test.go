package jaws

import (
	"errors"
	"fmt"
	"html/template"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestTagExpand(t *testing.T) {
	var av atomic.Value
	tests := []struct {
		name string
		tag  interface{}
		want []interface{}
	}{
		{
			name: "nil",
			tag:  nil,
			want: nil,
		},
		{
			name: "Tag",
			tag:  Tag("foo"),
			want: []interface{}{Tag("foo")},
		},
		{
			name: "TagGetter",
			tag:  atomicGetter{&av},
			want: []interface{}{&av},
		},
		{
			name: "[]Tag",
			tag:  []Tag{Tag("a"), Tag("b"), Tag("c")},
			want: []interface{}{Tag("a"), Tag("b"), Tag("c")},
		},
		{
			name: "[]interface{}",
			tag:  []interface{}{Tag("a"), &av},
			want: []interface{}{Tag("a"), &av},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TagExpand(nil, tt.tag, nil); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TagExpand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTagExpand_IllegalTypesPanic(t *testing.T) {
	tags := []any{
		string("string"),
		template.HTML("template.HTML"),
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
	}
	for _, tag := range tags {
		t.Run(fmt.Sprintf("%T", tag), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.FailNow()
				}
			}()
			TagExpand(nil, tag, nil)
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
		if e != ErrTooManyTags {
			t.Fail()
		}
	}()
	TagExpand(nil, tags, nil)
	t.FailNow()
}
