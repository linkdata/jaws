package jaws

import (
	"html/template"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/linkdata/deadlock"
)

type testStringSetter struct {
	mu        deadlock.Mutex
	s         string
	err       error
	setCount  int
	setCalled chan struct{}
	getCount  int
	getCalled chan struct{}
}

func newTestStringSetter(s string) *testStringSetter {
	return &testStringSetter{
		s:         s,
		setCalled: make(chan struct{}),
		getCalled: make(chan struct{}),
	}
}

func (ss *testStringSetter) Get() (s string) {
	ss.mu.Lock()
	s = ss.s
	ss.mu.Unlock()
	return
}

func (ss *testStringSetter) Set(s string) {
	ss.mu.Lock()
	ss.s = s
	ss.mu.Unlock()
}

func (ss *testStringSetter) SetCount() (n int) {
	ss.mu.Lock()
	n = ss.setCount
	ss.mu.Unlock()
	return
}

func (ss *testStringSetter) GetCount() (n int) {
	ss.mu.Lock()
	n = ss.getCount
	ss.mu.Unlock()
	return
}

func (ss *testStringSetter) JawsGetString(e *Element) (s string) {
	ss.mu.Lock()
	ss.getCount++
	if ss.getCount == 1 {
		close(ss.getCalled)
	}
	s = ss.s
	ss.mu.Unlock()
	return
}

func (ss *testStringSetter) JawsSetString(e *Element, s string) (err error) {
	ss.mu.Lock()
	ss.setCount++
	if ss.setCount == 1 {
		close(ss.setCalled)
	}
	if err = ss.err; err == nil {
		ss.s = s
	}
	ss.mu.Unlock()
	return
}

var _ StringSetter = (*testStringSetter)(nil)

func Test_makeStringGetter_panic(t *testing.T) {
	defer func() {
		if x := recover(); x != nil {
			if err, ok := x.(error); ok {
				if strings.Contains(err.Error(), "uint32") {
					return
				}
			}
		}
		t.Fail()
	}()
	makeStringGetter(uint32(42))
}

func Test_makeStringGetter(t *testing.T) {
	val := "<span>"
	var av atomic.Value
	av.Store(val)

	tests := []struct {
		name string
		v    interface{}
		want StringGetter
		out  string
		tag  interface{}
	}{
		{
			name: "StringGetter",
			v:    stringGetter{val},
			want: stringGetter{val},
			out:  val,
			tag:  nil,
		},
		{
			name: "string",
			v:    val,
			want: stringGetter{val},
			out:  val,
			tag:  nil,
		},
		{
			name: "template.HTML",
			v:    template.HTML(val),
			want: stringGetter{val},
			out:  val,
			tag:  nil,
		},
		{
			name: "*atomic.Value",
			v:    &av,
			want: atomicGetter{&av},
			out:  val,
			tag:  &av,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeStringGetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeStringGetter() = %v, want %v", got, tt.want)
			}
			if txt := got.JawsGetString(nil); txt != tt.out {
				t.Errorf("makeStringGetter().JawsGetString() = %v, want %v", txt, tt.out)
			}
			if tag := got.(TagGetter).JawsGetTag(nil); tag != tt.tag {
				t.Errorf("makeStringGetter().JawsGetTag() = %v, want %v", tag, tt.tag)
			}
		})
	}
}
