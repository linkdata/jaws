package jaws

import (
	"fmt"
	"html/template"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

var _ BoolSetter = (atomicSetter{})
var _ FloatSetter = (atomicSetter{})
var _ StringSetter = (atomicSetter{})
var _ TimeSetter = (atomicSetter{})
var _ HtmlGetter = (atomicSetter{})

func Test_atomicSetter_UninitializedDefaults(t *testing.T) {
	var av atomic.Value
	g := atomicSetter{v: &av}

	if g.JawsGetBool(nil) != false {
		t.Fail()
	}
	if g.JawsGetFloat(nil) != 0 {
		t.Fail()
	}
	if g.JawsGetString(nil) != "" {
		t.Fail()
	}
	if !g.JawsGetTime(nil).IsZero() {
		t.Fail()
	}
	if g.JawsGetHtml(nil) != "" {
		t.Fail()
	}
}

func Test_atomicSetter_bool(t *testing.T) {
	var av atomic.Value
	g := atomicSetter{v: &av}
	val := true
	g.JawsSetBool(nil, val)
	if g.JawsGetBool(nil) != val {
		t.Fail()
	}
}

func Test_atomicSetter_float64(t *testing.T) {
	var av atomic.Value
	g := atomicSetter{v: &av}
	val := float64(1.2)
	g.JawsSetFloat(nil, val)
	if g.JawsGetFloat(nil) != val {
		t.Fail()
	}
}

func Test_atomicSetter_string(t *testing.T) {
	var av atomic.Value
	g := atomicSetter{v: &av}
	val := "str"
	g.JawsSetString(nil, val)
	if g.JawsGetString(nil) != val {
		t.Fail()
	}
}

func Test_atomicSetter_time(t *testing.T) {
	var av atomic.Value
	g := atomicSetter{v: &av}
	val := time.Now()
	g.JawsSetTime(nil, val)
	if g.JawsGetTime(nil) != val {
		t.Fail()
	}
}

func Test_atomicSetter_JawsGetHtml(t *testing.T) {
	tests := []struct {
		name string
		av   atomic.Value
		v    any
		want template.HTML
	}{
		{
			name: "html",
			v:    template.HTML("html"),
			want: "html",
		},
		{
			name: "bool",
			v:    bool(true),
			want: "true",
		},
		{
			name: "float64",
			v:    float64(1.2),
			want: "1.2",
		},
		{
			name: "time.Time",
			v:    time.Now().Round(time.Minute),
			want: template.HTML(fmt.Sprint(time.Now().Round(time.Minute))),
		},
		{
			name: "html-escaped string",
			v:    "<span>",
			want: "&lt;span&gt;",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.av.Store(tt.v)
			g := atomicSetter{v: &tt.av}
			if got := g.JawsGetHtml(nil); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("atomicGetter.JawsGetHtml() for %#v = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}
